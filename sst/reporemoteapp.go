// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

// This file contains the code of the implementation of the server code of the remote SST Repository.
// The code for the application specific query that extracts data from a namedGraph
// (to be included in a bleve index) in located in package defaultderive.
// There might be other application specific derive packages.

package sst

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/semanticstep/sst-core/bboltproto"
	"github.com/semanticstep/sst-core/bleveproto"
	"github.com/semanticstep/sst-core/sstauth"
	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/document"
	"github.com/google/uuid"
	grpc_auth "github.com/grpc-ecosystem/go-grpc-middleware/auth"
	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

// repositoryMethodRoles maps gRPC method names to the minimum AccessMode
// required to invoke them. AccessMode values are ordered so that higher levels
// imply lower ones: SuperAdmin >= Admin >= ReadWrite >= ReadOnly.
var repositoryMethodRoles = map[string]sstauth.AccessMode{
	// ===== package sst.repository =====
	// RepoManagerService (SuperRepository operations require SuperAdmin)
	"/sst.repository.RepoManagerService/ListRepos":       sstauth.AccessMode_SuperAdmin,
	"/sst.repository.RepoManagerService/CreateRepo":      sstauth.AccessMode_SuperAdmin,
	"/sst.repository.RepoManagerService/DeleteRepo":      sstauth.AccessMode_SuperAdmin,
	"/sst.repository.RepoManagerService/GetRepoQuota":    sstauth.AccessMode_SuperAdmin,
	"/sst.repository.RepoManagerService/SetRepoQuota":    sstauth.AccessMode_SuperAdmin,
	"/sst.repository.RepoManagerService/GetSuperQuota":   sstauth.AccessMode_SuperAdmin,
	"/sst.repository.RepoManagerService/SetSuperQuota":   sstauth.AccessMode_SuperAdmin,
	"/sst.repository.RepoManagerService/GetMaxRepoCount": sstauth.AccessMode_SuperAdmin,
	"/sst.repository.RepoManagerService/SetMaxRepoCount": sstauth.AccessMode_SuperAdmin,

	// DatasetService (read)
	"/sst.repository.DatasetService/GetBranches":                 sstauth.AccessMode_ReadOnly,
	"/sst.repository.DatasetService/GetLeafCommits":              sstauth.AccessMode_ReadOnly,
	"/sst.repository.DatasetService/GetBleveInfo":                sstauth.AccessMode_ReadOnly,
	"/sst.repository.DatasetService/ListDatasets":                sstauth.AccessMode_ReadOnly,
	"/sst.repository.DatasetService/GetDataset":                  sstauth.AccessMode_ReadOnly,
	"/sst.repository.DatasetService/FetchDatasets":               sstauth.AccessMode_ReadOnly,
	"/sst.repository.DatasetService/GetRepositoryInfo":           sstauth.AccessMode_ReadOnly,
	"/sst.repository.DatasetService/GetRepositoryLog":            sstauth.AccessMode_ReadOnly,
	"/sst.repository.DatasetService/Document":                    sstauth.AccessMode_ReadOnly,
	"/sst.repository.DatasetService/Documents":                   sstauth.AccessMode_ReadOnly,
	"/sst.repository.DatasetService/DownloadNamedGraphRevision":  sstauth.AccessMode_ReadOnly,
	"/sst.repository.DatasetService/FindCommonParentRevision":    sstauth.AccessMode_ReadOnly,
	"/sst.repository.DatasetService/GetCommitForDatasetRevision": sstauth.AccessMode_ReadOnly,
	"/sst.repository.DatasetService/SyncTo":                      sstauth.AccessMode_ReadOnly,

	// DatasetService (write)
	"/sst.repository.DatasetService/CreateDataset": sstauth.AccessMode_ReadWrite,
	"/sst.repository.DatasetService/SetBranch":     sstauth.AccessMode_ReadWrite,
	"/sst.repository.DatasetService/PushDatasets":  sstauth.AccessMode_ReadWrite,
	"/sst.repository.DatasetService/DocumentSet":   sstauth.AccessMode_ReadWrite,
	"/sst.repository.DatasetService/SyncFrom":      sstauth.AccessMode_ReadWrite,

	// DatasetService (high-risk write)
	"/sst.repository.DatasetService/RemoveBranch":   sstauth.AccessMode_ReadWrite,
	"/sst.repository.DatasetService/DocumentDelete": sstauth.AccessMode_ReadWrite,

	// RefService (read)
	"/sst.repository.RefService/ListRefs": sstauth.AccessMode_ReadOnly,
	"/sst.repository.RefService/GetRef":   sstauth.AccessMode_ReadOnly,

	// CommitService
	"/sst.repository.CommitService/ListCommits":           sstauth.AccessMode_ReadOnly,
	"/sst.repository.CommitService/GetCommit":             sstauth.AccessMode_ReadOnly,
	"/sst.repository.CommitService/CompareCommits":        sstauth.AccessMode_ReadOnly,
	"/sst.repository.CommitService/GetCommitDetailsBatch": sstauth.AccessMode_ReadOnly,
	"/sst.repository.CommitService/CreateCommit":          sstauth.AccessMode_ReadWrite,

	// ===== package sst.ssquery =====
	"/sst.ssquery.IndexService/Search": sstauth.AccessMode_ReadOnly,
}

func stagePreCommitConstraints(
	repo Repository, index bleve.Index, batchUpdateBatched *sync.Map, stage Stage,
) (postCommitNotifier, error) {
	si, err := updateStageIndex(batchUpdateBatched, repo, stage)
	if err != nil {
		return nil, err
	}
	if si == nil {
		return nil, nil
	}
	return commitDocuments{repositoryIndex: repo.Bleve(), stageIndex: si}, nil
}

func updateStageIndex(batchUpdateBatched *sync.Map, repo Repository, stage Stage) (bleve.Index, error) {
	// stageID := stage.ID()
	// if stageID == BatchPostCommitNotificationStageID() {
	// 	batchVal, batchValLoaded := batchUpdateBatched.Load(stage)
	// 	var batch *indexBatch
	// 	if batchValLoaded {
	// 		batch = batchVal.(*indexBatch)
	// 	} else {
	// 		si, err := bleve.New("", repo.Bleve().Mapping())
	// 		if err != nil {
	// 			return nil, err
	// 		}
	// 		batch = &indexBatch{index: si, repo: repo, batch: si.NewBatch()}
	// 		batchVal, _ := batchUpdateBatched.LoadOrStore(stage, batch)
	// 		batch = batchVal.(*indexBatch)
	// 	}
	// 	// dgi, err := stage.Imports()
	// 	// if err != nil {
	// 	// 	return nil, err
	// 	// }
	// 	if len(dgi.DirectImports()) == 0 {
	// 		batchVal, loaded := batchUpdateBatched.LoadAndDelete(stage)
	// 		batch = batchVal.(*indexBatch)
	// 		if loaded {
	// 			err := indexAndCheckPreConditions(repo, batch)
	// 			if err != nil {
	// 				return nil, err
	// 			}
	// 			return batch.index, nil
	// 		}
	// 		return nil, nil
	// 	}
	// 	for _, ngi := range dgi.DirectImports() {
	// 		graph, err := ngi.Graph()
	// 		if err != nil {
	// 			return nil, err
	// 		}
	// 		_, brName := ngi.At()
	// 		if brName == DefaultBranch {
	// 			err = updateIndexForGraph(graph, batch, true)
	// 			if err != nil {
	// 				return nil, err
	// 			}
	// 		}
	// 	}
	// 	return nil, indexBatchIfOverThreshold(batch)
	// }

	// ephemeral := stage.Dataset().IsEphemeral()
	var graphIDs []uuid.UUID
	for _, ng := range stage.NamedGraphs() {
		graphIDs = append(graphIDs, ng.ID())
	}

	si, err := bleve.New("", repo.Bleve().Mapping())
	if err != nil {
		return nil, err
	}
	batch := &indexBatch{index: si, repo: repo, batch: si.NewBatch()}
	for _, graphID := range graphIDs {
		// if ephemeral && graphID == stageID {
		// 	continue
		// }
		// if graphID == stageID {
		if graphID == uuid.Nil {
			continue
		}
		graph := stage.NamedGraph(IRI(graphID.URN()))
		if graph == nil {
			return nil, err
		}
		// gi, err := stage.ImportsByID(graphID)
		// if err != nil {
		// 	return nil, err
		// }
		// _, brName := gi.At()
		brName := DefaultBranch
		if brName == DefaultBranch {
			err = updateIndexForGraph(graph, batch, true)
			if err != nil {
				return nil, err
			}
		}
		err = indexBatchIfOverThreshold(batch)
		if err != nil {
			return nil, err
		}
	}
	err = indexAndCheckPreConditions(repo, batch)
	if err != nil {
		return nil, err
	}
	return batch.index, nil
}

type commitDocuments struct {
	repositoryIndex bleve.Index
	stageIndex      bleve.Index
}

func (c commitDocuments) postCommitNotify() {
	docCnt, err := c.stageIndex.DocCount()
	if err != nil {
		panic(err)
	}
	if docCnt > 0 {
		im := c.repositoryIndex.Mapping()
		stageIdx, err := c.stageIndex.Advanced()
		if err != nil {
			panic(err)
		}
		r, err := stageIdx.Reader()
		if err != nil {
			panic(err)
		}
		defer multierr.AppendFunc(&err, func() error { return r.Close() })
		dr, err := r.DocIDReaderAll()
		if err != nil {
			panic(err)
		}
		defer multierr.AppendFunc(&err, func() error { return dr.Close() })
		batch := c.repositoryIndex.NewBatch()
		for {
			id, err := dr.Next()
			if err != nil {
				panic(err)
			}
			if len(id) == 0 {
				break
			}
			eid, err := r.ExternalID(id)
			if err != nil {
				panic(err)
			}
			rDoc, err := r.Document(eid)
			if err != nil {
				panic(err)
			}
			mDoc := rDoc.(*document.Document)
			var origDataBytes []byte
			for _, fl := range mDoc.Fields {
				if fl.Name() == "_original" {
					origDataBytes = fl.Value()
					break
				}
			}
			var data map[string]any
			err = json.Unmarshal(origDataBytes, &data)
			if err != nil {
				panic(err)
			}
			doc := document.NewDocument(rDoc.ID())
			err = im.MapDocument(doc, data)
			if err != nil {
				panic(err)
			}
			err = batch.IndexAdvanced(doc)
			if err != nil {
				panic(err)
			}
		}
		err = c.repositoryIndex.Batch(batch)
		if err != nil {
			panic(err)
		}
		return
	}
}

// This file contains how to handle incoming command-line arguments and start the server.
var recoveryOpt grpc_recovery.Option = grpc_recovery.WithRecoveryHandler(func(p interface{}) error {
	return newRecoveredError(p)
})

// RepositoryServer represents a SST Repository server instance.
// Should have a default Repository that always exists called "default".
type RepositoryServer struct {
	*grpc.Server
	// r Repository
	repoMap map[string]Repository
}

// Repository initial setup configuration.
type RepositoryServerConfig struct {
	RepoDir     string           // repository folder
	Issuer      string           // OIDC issuer URL
	ClientID    string           // OIDC client ID and SuperRepository URL
	ServerCert  *tls.Certificate // tls.Certificate
	Verbose     bool             // verbose log output, the value is a bool value, default false
	DeriveInfo  *SSTDeriveInfo   `json:"-"` // the bleve configuration(mapping)
	PerRepoAuth bool             // enable per-repository Keycloak client authorization
}

// NewServer creates and initializes a new RepositoryServer instance using the provided configuration.
// If the repository does not exist at the specified directory, it will be created automatically.
// If the repository exists, it will be opened.
// Typical use:
// // announces on the local network address
// lis, err := net.Listen("tcp", bindAddress)
//
//	if err != nil {
//		log.Fatalf("failed to listen: %v", err)
//	}
//
// // create SST server
// s, err := sst.NewServer(&config)
//
//	if err != nil {
//		return
//	}
//
//	// start serving SST repository service
//
//	if err := s.Serve(lis); err != nil {
//		log.Fatalf("failed to serve: %v", err)
//	}
func NewServer(c *RepositoryServerConfig) (*RepositoryServer, error) {
	// Single-repository mode does not use per-repository client authorization.
	c.PerRepoAuth = false

	repo, err := OpenLocalRepository(c.RepoDir, "default@semanticstep.net", "default")
	if err != nil {
		if err == ErrRepositoryDoesNotExist {
			repo, err = CreateLocalRepository(c.RepoDir, "default@semanticstep.net", "default", true)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	if err := repo.RegisterIndexHandler(c.DeriveInfo); err != nil {
		return nil, err
	}

	return newRepositoryServer(repo, c)
}

type indexServiceServer struct {
	r             Repository
	sr            SuperRepository
	superClientID string
	perRepoAuth   bool
	methodRoles   map[string]sstauth.AccessMode
	bleveproto.UnimplementedIndexServiceServer
}

func initIndexServiceServer(r Repository, sr SuperRepository, superClientID string, perRepoAuth bool, methodRoles map[string]sstauth.AccessMode) indexServiceServer {
	return indexServiceServer{r: r, sr: sr, superClientID: superClientID, perRepoAuth: perRepoAuth, methodRoles: methodRoles}
}

func indexServiceServerToRepository(i *indexServiceServer, ctx context.Context, repoName string) (Repository, error) {
	if repoName == "" {
		repoName = defaultSuperRepoName
	}
	var repo Repository
	var err error

	if i.sr != nil {
		repo, err = i.sr.Get(ctx, repoName)
		if err != nil {
			return nil, err
		}
	} else {
		repo = i.r
	}

	if repo == nil {
		return nil, ErrRepoNotFound
	}

	return repo, nil
}

func (i indexServiceServer) Search(stream bleveproto.IndexService_SearchServer) error {
	req, err := stream.Recv()
	if err != nil {
		return err
	}

	if i.perRepoAuth {
		repoURL := sstauth.PerRepoClientID(i.superClientID, sstauth.RepoNameFromRequest(req))
		if err := sstauth.CheckRepoAccess(
			stream.Context(),
			repoURL,
			"/sst.ssquery.IndexService/Search",
			i.methodRoles,
			i.superClientID,
			false,
		); err != nil {
			return err
		}
	}

	repo, err := indexServiceServerToRepository(&i, stream.Context(), req.RepoName)
	if err != nil {
		return err
	}

	res, err := searchEncoded(stream.Context(), repo.Bleve(), req.Request)
	if err != nil {
		return err
	}
	resResp, err := bleveproto.NewSearchResult(res)
	if err != nil {
		return err
	}
	msg := &grpc.PreparedMsg{}
	err = msg.Encode(stream, resResp)
	if err != nil {
		return err
	}

	return stream.SendMsg(msg)
}

func newRepositoryServer(r Repository, c *RepositoryServerConfig) (*RepositoryServer, error) {
	s := newServerWithConfig(c)

	dsService := datasetServiceServer{r: r, TimeNow: time.Now, clientID: c.ClientID}
	bboltproto.RegisterDatasetServiceServer(s, &dsService)
	GlobalLogger.Info("datasetService has been registered")

	refService := refServiceServer{R: r}
	bboltproto.RegisterRefServiceServer(s, &refService)
	GlobalLogger.Info("refService has been registered")

	commitService := commitServiceServer{r: r}
	bboltproto.RegisterCommitServiceServer(s, &commitService)
	GlobalLogger.Info("commitService has been registered")

	bleveproto.RegisterIndexServiceServer(s, initIndexServiceServer(r, nil, c.ClientID, c.PerRepoAuth, repositoryMethodRoles))
	GlobalLogger.Info("IndexService has been registered")

	reflection.Register(s)

	repoServer := &RepositoryServer{Server: s}
	repoServer.repoMap = make(map[string]Repository)
	repoServer.repoMap["default"] = r

	return repoServer, nil
}

func newServerWithConfig(config *RepositoryServerConfig) *grpc.Server {
	// Capacity: status + log + recovery + auth + rbac
	unaryInterceptors := make([]grpc.UnaryServerInterceptor, 0, 6)
	streamInterceptors := make([]grpc.StreamServerInterceptor, 0, 6)

	// 0) Base interceptors
	unaryInterceptors = append(unaryInterceptors, statusConverterUnaryServerInterceptor())
	streamInterceptors = append(streamInterceptors, statusConverterStreamServerInterceptor())

	if config != nil && config.Verbose {
		unaryInterceptors = append(unaryInterceptors, logUnaryServerInterceptor())
		streamInterceptors = append(streamInterceptors, logStreamServerInterceptor())
	}

	unaryInterceptors = append(unaryInterceptors, grpc_recovery.UnaryServerInterceptor(recoveryOpt))
	streamInterceptors = append(streamInterceptors, grpc_recovery.StreamServerInterceptor(recoveryOpt))

	// 1) Auth (token presence/validation) and RBAC (roles per method)
	if config != nil && config.Issuer != "" {
		var authFunction grpc_auth.AuthFunc
		// for local test using, when the server is running locally
		// the issuer can be set as "test://issuer", then it will use local testAuthFunc
		// to authenticate user.
		if config.Issuer == "test://issuer" {
			authFunction = sstauth.AuthFunc(config.Issuer, testAuthFunc)
		} else {
			// Issuer - keycloak URL
			authFunction = sstauth.AuthFunc(config.Issuer, nil)
		}
		unaryInterceptors = append(unaryInterceptors, grpc_auth.UnaryServerInterceptor(authFunction))
		streamInterceptors = append(streamInterceptors, grpc_auth.StreamServerInterceptor(authFunction))

		// Build OIDC verifier + attach RBAC only for real issuer
		if config.Issuer != "test://issuer" {
			if config.ClientID == "" {
				// Fail-fast is consistent with your previous log.Fatalf usage.
				log.Fatalf("init verifier failed: %v", errors.New("clientID required"))
			}

			ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel()

			verifier, err := sstauth.NewOIDCVerifier(ctx, config.Issuer, config.ClientID)
			if err != nil {
				log.Fatalf("init verifier failed: %v", err)
			}

			unaryInterceptors = append(
				unaryInterceptors,
				sstauth.UnaryRBACInterceptor(verifier, config.ClientID, config.PerRepoAuth, repositoryMethodRoles),
			)
			streamInterceptors = append(
				streamInterceptors,
				sstauth.StreamRBACInterceptor(verifier, config.ClientID, config.PerRepoAuth, repositoryMethodRoles),
			)
		}
	}

	// 2) Server options: build chain ONCE
	opts := make([]grpc.ServerOption, 0, 6)
	opts = append(
		opts,
		grpc.ChainUnaryInterceptor(unaryInterceptors...),
		grpc.ChainStreamInterceptor(streamInterceptors...),
		grpc.MaxRecvMsgSize(10*1024*1024),
	)

	// 3) TLS (optional)
	if config != nil && config.ServerCert != nil {
		opts = append(opts, grpc.Creds(credentials.NewServerTLSFromCert(config.ServerCert)))
		GlobalLogger.Info("gRPC server with TLS enabled")
	}

	return grpc.NewServer(opts...)
}

func testAuthFunc(ctx context.Context, rawToken, _ string) (info sstauth.SstUserInfo, _ error) {
	switch rawToken {
	case "test-token-1":
		return sstauth.SstUserInfo{
			Email: "test1@semanticstep.net",
		}, nil
	case "test-token-2":
		return sstauth.SstUserInfo{
			Email: "test2@semanticstep.net",
		}, nil
	}

	// Also accept unsigned JWT-shaped tokens used by tests that need role claims.
	// Signature is not verified here; downstream handlers re-parse the payload
	// with extractClientRolesNoVerify to obtain OIDC roles.
	if parts := strings.Split(rawToken, "."); len(parts) == 3 {
		payload, err := base64.RawURLEncoding.DecodeString(parts[1])
		if err == nil {
			var claims struct {
				Email string `json:"email"`
			}
			if err := json.Unmarshal(payload, &claims); err == nil && claims.Email != "" {
				return sstauth.SstUserInfo{Email: claims.Email}, nil
			}
		}
	}

	return info, status.Errorf(codes.Unauthenticated, "unrecognized token %v", rawToken)
}

// GracefulStopAndClose gracefully stops the gRPC server and closes the repository.
func (s RepositoryServer) GracefulStopAndClose() error {
	GlobalLogger.Info("gRPC server call GracefulStopAndClose")
	s.GracefulStop()

	var err error
	for _, repo := range s.repoMap {
		err = repo.Close()
	}
	return err
}

func statusConverterUnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (_ interface{}, err error) {
		resp, err := handler(ctx, req)
		var rErr recoveredError
		if errors.As(err, &rErr) {
			err = recoveredErrorToStatusError(rErr)
		}
		return resp, err
	}
}

func statusConverterStreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
		err = handler(srv, stream)
		var rErr recoveredError
		if errors.As(err, &rErr) {
			err = recoveredErrorToStatusError(rErr)
		}
		return err
	}
}

func recoveredErrorToStatusError(rErr recoveredError) error {
	stackEntries := strings.Split(rErr.Traceback, "\n")
	pStatus, err := status.Newf(codes.Internal, "%v", rErr.Error()).WithDetails(&errdetails.DebugInfo{
		StackEntries: stackEntries,
	})
	if err != nil {
		return status.Errorf(codes.Internal, "%v: %v", rErr.Error(), err)
	}
	return pStatus.Err()
}

func logUnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (_ interface{}, err error) {
		resp, err := handler(ctx, req)
		var rErr recoveredError
		if errors.As(err, &rErr) {
			fmt.Fprintln(os.Stderr, info.FullMethod)
			fmt.Fprintln(os.Stderr, rErr.IncludingTraceback().Error())
		}
		return resp, err
	}
}

func logStreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
		err = handler(srv, stream)
		var rErr recoveredError
		if errors.As(err, &rErr) {
			fmt.Fprintln(os.Stderr, info.FullMethod)
			fmt.Fprintln(os.Stderr, rErr.IncludingTraceback().Error())
		}
		return err
	}
}

// for gRPC server - indexServiceServer using.
func searchEncoded(
	ctx context.Context, index bleve.Index, encReq []byte,
) (*bleve.SearchResult, error) {
	req, err := decodeSearchRequest(encReq)
	if err != nil {
		return nil, err
	}
	outputReq, err := json.MarshalIndent(req, "", "  ")
	if err != nil {
		GlobalLogger.Error("error marshaling search request JSON", zap.Error(err))
		return nil, err
	}
	GlobalLogger.Debug("search request", zap.String("request", string(outputReq)))
	return index.SearchInContext(ctx, req)
}

func decodeSearchRequest(encReq []byte) (*bleve.SearchRequest, error) {
	var request *bleve.SearchRequest
	err := json.Unmarshal(encReq, &request)
	return request, err
}

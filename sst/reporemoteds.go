// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/semanticstep/sst-core/bboltproto"
	"github.com/semanticstep/sst-core/sstauth"
	"go.etcd.io/bbolt"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// var sstTokenized = Tokenized(internal.Token{})

const (
	defaultPageSize = 100
	maximumPageSize = 1000
)

type datasetServiceServer struct {
	r        Repository
	sr       SuperRepository
	clientID string
	bboltproto.UnimplementedDatasetServiceServer
	TimeNow func() time.Time
}

func (s *datasetServiceServer) GetBleveInfo(
	ctx context.Context,
	request *bboltproto.GetRepoBleveInfoRequest,
) (*bboltproto.GetRepoBleveInfoResponse, error) {
	GlobalLogger.Debug("gRPC datasetServiceServer received GetBleveInfo request")
	if request == nil {
		panic("request is nil")
	}

	repo, err := datasetServiceServerToRepository(s, ctx, defaultSuperRepoName)
	if err != nil {
		return nil, err
	}

	resp := bboltproto.GetRepoBleveInfoResponse{
		Name:    repo.(*localFullRepository).config.deriveInfo.DerivePackageName,
		Version: repo.(*localFullRepository).config.deriveInfo.DerivePackageVersion,
	}

	return &resp, nil
}

func (s *datasetServiceServer) GetRepositoryInfo(
	ctx context.Context,
	request *bboltproto.GetRepositoryInfoRequest,
) (*bboltproto.GetRepositoryInfoResponse, error) {
	GlobalLogger.Info("gRPC datasetServiceServer received GetRepositoryInfo request", zap.String("RepoName", request.RepoName), zap.String("BranchName", request.BranchName))
	if request == nil {
		return nil, fmt.Errorf("request is nil")
	}

	repo, err := datasetServiceServerToRepository(s, ctx, request.RepoName)
	if err != nil {
		return nil, ErrRepoNotFound
	}

	stats, err := repo.Info(ctx, request.BranchName)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository info: %w", err)
	}

	roles := make([]string, 0)
	tok, err := tokenFromIncomingContext(ctx)
	if err != nil {
		if status.Code(err) == codes.Unauthenticated {
			GlobalLogger.Debug("no token found in context", zap.Error(err))
		} else {
			return nil, err
		}
	} else {
		extractedRoles, err := extractClientRolesNoVerify(tok, s.clientID)
		if err != nil {
			// GlobalLogger.Error("failed to extract client roles", zap.Error(err))
			GlobalLogger.Debug("failed to extract client roles", zap.Error(err))
		}
		roles = append(roles, extractedRoles...)
	}
	// log.Println("extracted token:", tok)

	// log.Println("extracted roles:", roles)

	mode := sstauth.AccessModeFromRoles(roles)
	isAdmin := sstauth.HasAccess(mode, sstauth.AccessMode_Admin)
	resp := &bboltproto.GetRepositoryInfoResponse{
		AccessRight:                 mode.String(),
		BboltSize:                   stats.MasterDBSize,
		BleveSize:                   stats.DerivedDBSize,
		NumberOfDatasets:            stats.NumberOfDatasets,
		NumberOfDatasetsInBranch:    stats.NumberOfDatasetsInBranch,
		NumberOfDatasetRevisions:    stats.NumberOfDatasetRevisions,
		NumberOfNamedGraphRevisions: stats.NumberOfNamedGraphRevisions,
		NumberOfCommits:             stats.NumberOfCommits,
		NumberOfRepositoryLogs:      stats.NumberOfRepositoryLogs,
		DocumentDbSize:              stats.DocumentDBSize,
		NumberOfDocuments:           stats.NumberOfDocuments,
		VersionHash:                 stats.VersionHash,
	}
	if isAdmin {
		resp.ActualRepositorySize = stats.ActualRepositorySize
		resp.MaxRepositorySize = stats.MaxRepositorySize
	}

	GlobalLogger.Info("gRPC datasetServiceServer GetRepositoryInfo response")
	return resp, nil
}

func tokenFromIncomingContext(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "missing metadata")
	}

	// gRPC metadata keys are lowercase
	vals := md.Get("authorization")
	if len(vals) == 0 {
		return "", status.Error(codes.Unauthenticated, "missing authorization")
	}

	// Expect: "Bearer <token>"
	v := strings.TrimSpace(vals[0])
	parts := strings.SplitN(v, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", status.Error(codes.Unauthenticated, "invalid authorization format")
	}
	if parts[1] == "" {
		return "", status.Error(codes.Unauthenticated, "empty token")
	}
	return parts[1], nil
}

type infoClaims struct {
	ResourceAccess map[string]struct {
		Roles []string `json:"roles"`
	} `json:"resource_access"`
}

func extractClientRolesNoVerify(rawAccessToken, targetClientID string) ([]string, error) {
	parts := strings.Split(rawAccessToken, ".")
	if len(parts) < 2 {
		return nil, errors.New("not a jwt")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode payload: %w", err)
	}

	var c infoClaims
	if err := json.Unmarshal(payload, &c); err != nil {
		return nil, fmt.Errorf("unmarshal claims: %w", err)
	}

	ra, ok := c.ResourceAccess[targetClientID]
	if !ok {
		return nil, fmt.Errorf("no resource_access for clientId=%q", targetClientID)
	}
	return ra.Roles, nil
}

func (s *datasetServiceServer) GetRepositoryLog(
	ctx context.Context,
	req *bboltproto.GetRepositoryLogRequest,
) (*bboltproto.GetRepositoryLogResponse, error) {
	repo, err := datasetServiceServerToRepository(s, ctx, req.RepoName)
	if err != nil {
		return nil, err
	}

	var startPtr, endPtr *int

	if req.Start != nil {
		startVal := int(*req.Start)
		startPtr = &startVal
	}
	if req.End != nil {
		endVal := int(*req.End)
		endPtr = &endVal
	}

	logs, err := repo.Log(ctx, startPtr, endPtr)
	if err != nil {
		return nil, err
	}

	resp := &bboltproto.GetRepositoryLogResponse{}
	for _, entry := range logs {
		resp.Entries = append(resp.Entries, &bboltproto.RepositoryLogEntry{
			LogKey: entry.LogKey,
			Fields: entry.Fields,
		})
	}

	return resp, nil
}

func (s *datasetServiceServer) DocumentSet(stream bboltproto.DatasetService_DocumentSetServer) error {
	var (
		reader = &bytes.Buffer{}
		info   *bboltproto.DocumentInfo
		repo   Repository
	)

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return status.Errorf(codes.Unknown, "failed to receive document upload chunk: %v", err)
		}
		repo, err = datasetServiceServerToRepository(s, stream.Context(), req.RepoName)
		if err != nil {
			return status.Errorf(codes.Unavailable, "repository is not available: %v", err)
		}

		switch x := req.Event.(type) {
		case *bboltproto.DocumentSetRequest_Info:
			info = x.Info
		case *bboltproto.DocumentSetRequest_Content:
			if _, err := reader.Write(x.Content); err != nil {
				return status.Errorf(codes.Internal, "failed to write content chunk: %v", err)
			}
		default:
			return status.Error(codes.InvalidArgument, "invalid upload request: missing info or content")
		}
	}

	if info == nil {
		return status.Error(codes.InvalidArgument, "document info must be provided before content")
	}

	// Call repository.Upload
	hash, err := repo.DocumentSet(stream.Context(), info.GetMimeType(), bufio.NewReader(reader))
	if err != nil {
		if errors.Is(err, ErrQuotaExceeded) {
			return status.Errorf(codes.ResourceExhausted, "upload failed: %v", err)
		}
		return status.Errorf(codes.Internal, "upload failed: %v", err)
	}

	// Send response with hash
	return stream.SendAndClose(&bboltproto.DocumentSetResponse{
		Hash: hash[:],
	})
}

func (s *datasetServiceServer) Document(
	req *bboltproto.DocumentRequest,
	stream bboltproto.DatasetService_DocumentServer,
) error {
	repo, err := datasetServiceServerToRepository(s, stream.Context(), req.RepoName)
	if err != nil {
		return err
	}

	localRepo, ok := repo.(*localFullRepository)
	if !ok {
		return status.Error(codes.Unavailable, "Repository is not available")
	}

	if len(req.GetHash()) != 32 {
		return status.Error(codes.InvalidArgument, "invalid hash length")
	}

	var hash Hash
	copy(hash[:], req.GetHash())

	// Open the file first to ensure it exists and to stat its size.
	vaultDir := filepath.Join(localRepo.config.repositoryDir, VaultDirName)
	fullPath := vaultPath(vaultDir, hash)

	file, err := os.Open(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return status.Error(codes.NotFound, "document file not found")
		}
		return status.Errorf(codes.Internal, "failed to open document file: %v", err)
	}
	defer file.Close()

	fi, err := file.Stat()
	if err != nil {
		return status.Errorf(codes.Internal, "failed to stat document file: %v", err)
	}

	// Read metadata from BoltDB.
	info, err := readDocumentInfo(localRepo.DB(), hash)
	if err != nil {
		return status.Errorf(codes.NotFound, "document info not found: %v", err)
	}

	// Send document info to the client.
	if err := stream.Send(&bboltproto.DocumentResponse{
		Event: &bboltproto.DocumentResponse_Info{
			Info: &bboltproto.DocumentInfo{
				Hash:      hash[:],
				MimeType:  info.MIMEType,
				Author:    info.Author,
				Timestamp: timestamppb.New(info.Timestamp),
				SizeBytes: fi.Size(),
			},
		},
	}); err != nil {
		return status.Errorf(codes.Internal, "failed to send document info: %v", err)
	}

	// Stream the file in chunks to the client.
	buf := make([]byte, 32*1024) // 32KB chunk size
	for {
		n, rerr := file.Read(buf)
		if rerr != nil && rerr != io.EOF {
			return status.Errorf(codes.Internal, "failed to read file: %v", rerr)
		}
		if n == 0 {
			break
		}
		if err := stream.Send(&bboltproto.DocumentResponse{
			Event: &bboltproto.DocumentResponse_Chunk{
				Chunk: buf[:n],
			},
		}); err != nil {
			return status.Errorf(codes.Internal, "failed to send chunk: %v", err)
		}
	}

	return nil
}

func (s *datasetServiceServer) Documents(
	ctx context.Context,
	req *bboltproto.DocumentsListRequest,
) (*bboltproto.DocumentsListResponse, error) {
	repo, err := datasetServiceServerToRepository(s, ctx, req.RepoName)
	if err != nil {
		return nil, err
	}
	docs, err := repo.Documents(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list documents: %v", err)
	}

	result := make([]*bboltproto.DocumentInfo, 0, len(docs))
	for _, doc := range docs {
		result = append(result, &bboltproto.DocumentInfo{
			Hash:      doc.Hash[:],
			MimeType:  doc.MIMEType,
			Author:    doc.Author,
			Timestamp: timestamppb.New(doc.Timestamp),
			SizeBytes: doc.Size,
		})
	}

	return &bboltproto.DocumentsListResponse{Documents: result}, nil
}

func (s *datasetServiceServer) DocumentDelete(
	ctx context.Context,
	req *bboltproto.DocumentDeleteRequest,
) (*bboltproto.DocumentDeleteResponse, error) {
	if len(req.Hash) != 32 {
		return nil, fmt.Errorf("invalid hash length: %d", len(req.Hash))
	}

	repo, err := datasetServiceServerToRepository(s, ctx, req.RepoName)
	if err != nil {
		return nil, err
	}

	var hash Hash
	copy(hash[:], req.Hash)

	err = repo.DocumentDelete(ctx, hash)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "document file not found for hash %s", hash.String())
	}

	return &bboltproto.DocumentDeleteResponse{}, nil
}

func (s *datasetServiceServer) DownloadNamedGraphRevision(
	req *bboltproto.DownloadNamedGraphRevisionRequest,
	stream bboltproto.DatasetService_DownloadNamedGraphRevisionServer,
) error {
	var hash Hash
	copy(hash[:], req.GetHash())

	repo, err := datasetServiceServerToRepository(s, stream.Context(), req.RepoName)
	if err != nil {
		return err
	}

	var content []byte
	err = repo.(*localFullRepository).DB().View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("ngr"))
		if bucket == nil {
			return fmt.Errorf("bucket 'ngr' not found")
		}
		data := bucket.Get(hash[:])
		if data == nil {
			return fmt.Errorf("namedGraphRevision %x not found", hash)
		}
		content = make([]byte, len(data))
		copy(content, data)
		return nil
	})
	if err != nil {
		return status.Errorf(codes.NotFound, "read failed: %v", err)
	}

	const chunkSize = 32 * 1024
	for i := 0; i < len(content); i += chunkSize {
		end := i + chunkSize
		if end > len(content) {
			end = len(content)
		}
		err := stream.Send(&bboltproto.DownloadNamedGraphRevisionResponse{
			Chunk: content[i:end],
		})
		if err != nil {
			return status.Errorf(codes.Internal, "stream send error: %v", err)
		}
	}

	return nil
}

func (s *datasetServiceServer) FindCommonParentRevision(
	ctx context.Context,
	request *bboltproto.FindCommonParentRevisionRequest,
) (*bboltproto.FindCommonParentRevisionResponse, error) {
	GlobalLogger.Debug("gRPC datasetServiceServer received FindCommonParentRevision request")

	repo, err := datasetServiceServerToRepository(s, ctx, request.RepoName)
	if err != nil {
		return nil, err
	}

	ds, err := repo.Dataset(ctx, IRI(uuid.UUID(request.DatasetId).URN()))
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "dataset not found: %v", err)
	}

	commonHash, err := ds.FindCommonParentRevision(ctx, Hash(request.DatasetCommitRv1), Hash(request.DatasetCommitRv2))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to find common parent revision: %v", err)
	}

	resp := bboltproto.FindCommonParentRevisionResponse{CommonParentCommitRv: commonHash[:]}

	return &resp, err
}

func (s *datasetServiceServer) GetCommitForDatasetRevision(
	ctx context.Context,
	request *bboltproto.GetCommitForDatasetRevisionRequest,
) (*bboltproto.GetCommitForDatasetRevisionResponse, error) {
	GlobalLogger.Debug("gRPC datasetServiceServer received GetCommitForDatasetRevision request")

	repo, err := datasetServiceServerToRepository(s, ctx, request.RepoName)
	if err != nil {
		return nil, err
	}

	ds, err := repo.Dataset(ctx, IRI(uuid.UUID(request.DatasetId).URN()))
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "dataset not found: %v", err)
	}

	commitHash, err := ds.CommitForRevision(ctx, BytesToHash(request.DatasetRevision))
	if err != nil {
		if errors.Is(err, ErrDatasetRevisionNotFound) {
			return nil, status.Errorf(codes.NotFound, "%v", err)
		}
		return nil, status.Errorf(codes.Internal, "failed to get commit for dataset revision: %v", err)
	}

	resp := bboltproto.GetCommitForDatasetRevisionResponse{CommitHash: commitHash[:]}

	return &resp, nil
}

func (s *datasetServiceServer) GetBranches(
	ctx context.Context,
	request *bboltproto.GetBranchesRequest,
) (*bboltproto.GetBranchesResponse, error) {
	GlobalLogger.Debug(
		"GetBranches request Received",
		zap.String("RepoName", request.RepoName),
		zap.String("Dataset UUID", uuid.UUID(request.Uuid).String()),
	)
	repo, err := datasetServiceServerToRepository(s, ctx, request.RepoName)
	if err != nil {
		GlobalLogger.Error("", zap.Error(err))
		return nil, err
	}

	ds, err := repo.Dataset(ctx, IRI(uuid.UUID(request.Uuid).URN()))
	if err != nil {
		GlobalLogger.Error("", zap.Error(err))
		return nil, err
	}
	branchCommitHashMap, err := ds.Branches(ctx)

	var BranchRefs []*bboltproto.BranchRef

	for key, val := range branchCommitHashMap {
		BranchRefs = append(BranchRefs, &bboltproto.BranchRef{
			Branch:     key,
			CommitHash: val[:],
		})
	}

	resp := bboltproto.GetBranchesResponse{BranchRefs: BranchRefs}

	return &resp, err
}

func datasetServiceServerToRepository(s *datasetServiceServer, ctx context.Context, repoName string) (Repository, error) {
	if repoName == "" {
		repoName = defaultSuperRepoName
	}
	var repo Repository
	var err error

	if s.sr != nil {
		repo, err = s.sr.Get(ctx, repoName)
		if err != nil {
			return nil, err
		}
	} else {
		repo = s.r
	}

	if repo == nil {
		return nil, ErrRepoNotFound
	}

	return repo, nil
}

func (s *datasetServiceServer) SetBranch(
	ctx context.Context,
	request *bboltproto.SetBranchRequest,
) (*bboltproto.SetBranchResponse, error) {
	GlobalLogger.Debug(
		"SetBranch request Received",
		zap.String("RepoName", request.RepoName),
		zap.String("Branch", request.Branch),
		zap.String("Dataset UUID", uuid.UUID(request.Uuid).String()),
	)

	repo, err := datasetServiceServerToRepository(s, ctx, request.RepoName)

	ds, err := repo.Dataset(ctx, IRI(uuid.UUID(request.Uuid).URN()))
	if err != nil {
		GlobalLogger.Error("", zap.Error(err))
		return nil, err
	}
	err = ds.SetBranchCommit(ctx, Hash(request.CommitHash), request.Branch)
	if err != nil {
		return nil, err
	}

	// Ensure asynchronous Bleve index updates are visible before returning to
	// the remote client.
	FlushBleveIndex(repo)

	return &bboltproto.SetBranchResponse{}, nil
}

func (s *datasetServiceServer) RemoveBranch(
	ctx context.Context,
	request *bboltproto.RemoveBranchRequest,
) (*bboltproto.RemoveBranchResponse, error) {
	GlobalLogger.Debug(
		"RemoveBranch request Received",
		zap.String("RepoName", request.RepoName),
		zap.String("Branch", request.Branch),
		zap.String("Dataset UUID", uuid.UUID(request.Uuid).String()),
	)

	repo, err := datasetServiceServerToRepository(s, ctx, request.RepoName)
	if err != nil {
		GlobalLogger.Error("", zap.Error(err))
		return nil, err
	}

	ds, err := repo.Dataset(ctx, IRI(uuid.UUID(request.Uuid).URN()))
	if err != nil {
		GlobalLogger.Error("", zap.Error(err))
		return nil, err
	}
	err = ds.RemoveBranch(ctx, request.Branch)
	if err != nil {
		return nil, err
	}

	// Ensure asynchronous Bleve index updates are visible before returning to
	// the remote client.
	FlushBleveIndex(repo)

	return &bboltproto.RemoveBranchResponse{}, nil
}

func (s *datasetServiceServer) GetLeafCommits(
	ctx context.Context,
	request *bboltproto.GetLeafCommitsRequest,
) (*bboltproto.GetLeafCommitsResponse, error) {
	GlobalLogger.Debug(
		"RemoveBranch request Received",
		zap.String("RepoName", request.RepoName),
		zap.String("Dataset UUID", uuid.UUID(request.Uuid).String()),
	)

	repo, err := datasetServiceServerToRepository(s, ctx, request.RepoName)
	if err != nil {
		GlobalLogger.Error("", zap.Error(err))
		return nil, err
	}

	ds, err := repo.Dataset(ctx, IRI(uuid.UUID(request.Uuid).URN()))
	if err != nil {
		GlobalLogger.Error("", zap.Error(err))
		return nil, err
	}
	leafCommitHashes, err := ds.LeafCommits(ctx)
	if err != nil {
		GlobalLogger.Error("", zap.Error(err))
		return nil, err
	}

	var returnedLeafCommitHashes [][]byte

	for _, val := range leafCommitHashes {
		returnedLeafCommitHashes = append(returnedLeafCommitHashes, val[:])
	}

	resp := bboltproto.GetLeafCommitsResponse{CommitHashes: returnedLeafCommitHashes}

	return &resp, nil
}

func (s *datasetServiceServer) ListDatasets(
	ctx context.Context,
	request *bboltproto.ListDatasetsRequest,
) (*bboltproto.ListDatasetsResponse, error) {
	GlobalLogger.Debug("gRPC datasetServiceServer received ListDatasets request")

	repo, err := datasetServiceServerToRepository(s, ctx, request.RepoName)
	if err != nil {
		return nil, err
	}
	md, _ := metadata.FromIncomingContext(ctx)
	_ = md // TODO:
	var startDsID uuid.UUID
	if request.PageToken != "" {
		encPageToken, err := base64.URLEncoding.DecodeString(request.PageToken)
		if err != nil {
			return nil, err
		}
		var pageToken bboltproto.ListDatasetsResponse_NextPageToken
		err = proto.Unmarshal(encPageToken, &pageToken)
		if err != nil {
			return nil, err
		}
		copy(startDsID[:], pageToken.NextId)
	}
	var resp bboltproto.ListDatasetsResponse
	pageSize := request.PageSize
	if pageSize == 0 {
		pageSize = defaultPageSize
	} else if pageSize > maximumPageSize {
		pageSize = maximumPageSize
	}
	fmFilter := request.FieldMask_Filter()
	unmaskUUID, unmaskIRI := fmFilter.MaskedOut_Uuid(), fmFilter.MaskedOut_Iri()
	err = repo.(*localFullRepository).db.View(func(tx *bbolt.Tx) error {
		datasets := tx.Bucket(keyDatasets)
		if datasets == nil {
			return nil
		}
		c := datasets.Cursor()
		seek := c.First
		if startDsID != uuid.Nil {
			seek = func() (key []byte, value []byte) {
				return c.Seek(startDsID[:])
			}
		}
		resp.Datasets = make([]*bboltproto.Dataset, 0, pageSize)
		var i int32
		for k, _ := seek(); k != nil; k, _ = c.Next() {
			dsBucket := datasets.Bucket(k)
			dsID := uuid.UUID(*(*[len(uuid.UUID{})]byte)(k))
			i++
			if i > pageSize {
				encNextPageToken, err := proto.Marshal(&bboltproto.ListDatasetsResponse_NextPageToken{NextId: dsID[:]})
				if err != nil {
					return err
				}
				resp.NextPageToken = base64.URLEncoding.EncodeToString(encNextPageToken)
				break
			}
			var iriPtr *string
			if unmaskIRI {
				// Use getDatasetIRI to get the IRI from the dataset bucket
				dsIRI := getDatasetIRI(dsBucket, dsID)
				iriPtr = &dsIRI
			}
			var uuid []byte
			if unmaskUUID {
				uuid = dsID[:]
			}
			resp.Datasets = append(resp.Datasets, &bboltproto.Dataset{
				Uuid: uuid,
				Iri:  iriPtr,
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (s *datasetServiceServer) GetDataset(stream bboltproto.DatasetService_GetDatasetServer) error {
	request, err := stream.Recv()
	if err != nil {
		return err
	}

	repo, err := datasetServiceServerToRepository(s, stream.Context(), request.RepoName)
	if err != nil {
		return err
	}

	fmFilter := request.FieldMask_Filter()
	unmaskUUID, unmaskIRI := fmFilter.MaskedOut_Uuid(), fmFilter.MaskedOut_Iri()
	dsID, err := uuid.FromBytes(request.GetDatasetUuid())
	if err != nil {
		return err
	}

	GlobalLogger.Debug("GetDataset request Received", zap.String("Dataset UUID", dsID.String()))

	ds, err := repo.Dataset(context.TODO(), IRI(dsID.URN()))
	if err != nil {
		return err
	}
	var response bboltproto.Dataset
	if unmaskUUID {
		response.Uuid = dsID[:]
	}
	if unmaskIRI {
		dsIRI := ds.IRI().String()
		response.Iri = &dsIRI
	}
	return stream.SendAndClose(&response)
}

// not used
func (s *datasetServiceServer) CreateDataset(
	ctx context.Context,
	request *bboltproto.CreateDatasetRequest,
) (*bboltproto.Dataset, error) {
	log.Panic("gRPC datasetServiceServer received CreateDataset request not expected to be used")
	var response bboltproto.Dataset
	return &response, nil
}

func (s *datasetServiceServer) FetchDatasets(stream bboltproto.DatasetService_FetchDatasetsServer) error {
	GlobalLogger.Debug("gRPC datasetServiceServer received FetchDatasets request")
	objSender, err := newDatasetObjectSender(s.r, stream)
	if err != nil {
		return err
	}
	return objSender.sendObjects(stream.Context(), s, nil)
}

// SyncFrom handles the server-side sync operation.
func (s *datasetServiceServer) SyncFrom(stream bboltproto.DatasetService_SyncFromServer) error {
	GlobalLogger.Debug("gRPC datasetServiceServer received SyncFrom request")

	var repoName string
	var localRepo *localFullRepository
	var db *bbolt.DB
	var syncMetadata *bboltproto.SyncFromMetadata
	var existingDSRs map[Hash]struct{}

	stats := &bboltproto.SyncFromResponse{}

	// Track document_info hashes that were synced
	documentHashesToSync := make(map[string]bool)

	// Process streamed bucket data
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return status.Errorf(codes.Internal, "failed to receive sync data: %v", err)
		}

		switch data := req.Data.(type) {
		case *bboltproto.SyncFromRequest_Metadata:
			syncMetadata = data.Metadata
			// Extract repo_name from metadata for SuperRepository support
			if data.Metadata != nil {
				repoName = data.Metadata.RepoName
			}
			// Get the correct repository from SuperRepository
			if localRepo == nil {
				repo, err := datasetServiceServerToRepository(s, stream.Context(), repoName)
				if err != nil {
					return status.Errorf(codes.Internal, "failed to get repository: %v", err)
				}
				var ok bool
				localRepo, ok = repo.(*localFullRepository)
				if !ok {
					return status.Errorf(codes.Unimplemented, "SyncFrom is only supported for LocalFullRepository, got %T", repo)
				}
				db = localRepo.db
				var snapshotErr error
				existingDSRs, snapshotErr = snapshotExistingDSRs(db)
				if snapshotErr != nil {
					return status.Errorf(codes.Internal, "failed to snapshot existing dataset revisions: %v", snapshotErr)
				}
			}
			continue
		case *bboltproto.SyncFromRequest_BucketData:
			if db == nil {
				return status.Errorf(codes.FailedPrecondition, "metadata must be sent before bucket data")
			}
			bd := data.BucketData
			if err := s.processBucketData(db, bd, stats, documentHashesToSync); err != nil {
				return status.Errorf(codes.Internal, "failed to process bucket data: %v", err)
			}
		}
	}

	// Check and sync document files for all synced document_info
	// Note: Files should already be in the vault via DocumentSet, but we verify and log warnings if missing
	if localRepo != nil && len(documentHashesToSync) > 0 {
		if err := s.verifyDocumentFiles(localRepo, documentHashesToSync); err != nil {
			GlobalLogger.Warn("some document files may be missing in vault", zap.Error(err))
			// Don't fail the sync if files are missing, as they might have been uploaded separately
		}
	}

	if localRepo != nil && existingDSRs != nil {
		fromRepoURL := ""
		branchName := ""
		if syncMetadata != nil {
			fromRepoURL = syncMetadata.FromRepoUrl
			branchName = syncMetadata.BranchName
		}
		if err := writeSyncFromLogAfterSync(stream.Context(), localRepo, fromRepoURL, existingDSRs, branchName); err != nil {
			return status.Errorf(codes.Internal, "failed to write sync_from log entry: %v", err)
		}
	}

	// Send response with statistics
	return stream.SendAndClose(stats)
}

// SyncTo handles the server-side sync operation for remote to local sync.
// Server streams bucket data to the client.
func (s *datasetServiceServer) SyncTo(req *bboltproto.SyncToRequest, stream bboltproto.DatasetService_SyncToServer) error {
	GlobalLogger.Debug("gRPC datasetServiceServer received SyncTo request")

	// Get bbolt database from repository
	// Server-side repository should always be LocalFullRepository
	localRepo, ok := s.r.(*localFullRepository)
	if !ok {
		return status.Errorf(codes.Unimplemented, "SyncTo is only supported for LocalFullRepository, got %T", s.r)
	}
	db := localRepo.db

	// Extract dataset IDs and branch name from metadata and expand with import dependencies
	var datasetIDSet map[uuid.UUID]struct{}
	var branchName string
	if req.Metadata != nil {
		// Extract branch name
		if req.Metadata.BranchName != "" {
			branchName = req.Metadata.BranchName
		}

		// Extract and expand dataset IDs
		if len(req.Metadata.DatasetIds) > 0 {
			initialDatasetIDs := make([]uuid.UUID, len(req.Metadata.DatasetIds))
			for i, dsIDBytes := range req.Metadata.DatasetIds {
				var dsID uuid.UUID
				copy(dsID[:], dsIDBytes)
				initialDatasetIDs[i] = dsID
			}

			var err error
			datasetIDSet, err = collectAllImportedDatasets(stream.Context(), localRepo, initialDatasetIDs, branchName)
			if err != nil {
				return status.Errorf(codes.Internal, "failed to collect imported datasets: %v", err)
			}
		}
	}

	stats := &bboltproto.SyncToComplete{}

	// Stream all bucket data to client
	if err := s.streamBucketsToClient(db, stream, stats, datasetIDSet, branchName); err != nil {
		return status.Errorf(codes.Internal, "failed to stream bucket data: %v", err)
	}

	// Send completion message with statistics
	if err := stream.Send(&bboltproto.SyncToResponse{
		Data: &bboltproto.SyncToResponse_Complete{
			Complete: stats,
		},
	}); err != nil {
		return status.Errorf(codes.Internal, "failed to send completion message: %v", err)
	}

	return nil
}

func (s *datasetServiceServer) streamBucketsToClient(
	db *bbolt.DB,
	stream bboltproto.DatasetService_SyncToServer,
	stats *bboltproto.SyncToComplete,
	datasetIDSet map[uuid.UUID]struct{},
	branchName string,
) error {
	return db.View(func(tx *bbolt.Tx) error {
		// Get buckets needed for filtering
		var dsrBucket, datasetsBucket, commitsBucket *bbolt.Bucket
		if datasetIDSet != nil || !isAllBranches(branchName) {
			dsrBucket = tx.Bucket(keyDatasetRevisions)
			datasetsBucket = tx.Bucket(keyDatasets)
			commitsBucket = tx.Bucket(keyCommits)
		}

		// 1. Stream NamedGraphRevisions
		if err := s.streamNamedGraphRevisionsToClient(tx, stream, stats, datasetIDSet, dsrBucket, datasetsBucket, commitsBucket, branchName); err != nil {
			return fmt.Errorf("failed to stream NamedGraphRevisions: %w", err)
		}

		// 2. Stream DatasetRevisions
		if err := s.streamDatasetRevisionsToClient(tx, stream, stats, datasetIDSet, datasetsBucket, commitsBucket, branchName); err != nil {
			return fmt.Errorf("failed to stream DatasetRevisions: %w", err)
		}

		// 3. Stream Datasets
		if err := s.streamDatasetsToClient(tx, stream, stats, datasetIDSet, branchName); err != nil {
			return fmt.Errorf("failed to stream Datasets: %w", err)
		}

		// 4. Stream Commits
		if err := s.streamCommitsToClient(tx, stream, stats, datasetIDSet, branchName); err != nil {
			return fmt.Errorf("failed to stream Commits: %w", err)
		}

		// 5. Stream document_info
		if err := s.streamDocumentInfoToClient(tx, stream, stats, datasetIDSet); err != nil {
			return fmt.Errorf("failed to stream document_info: %w", err)
		}

		return nil
	})
}

func (s *datasetServiceServer) streamNamedGraphRevisionsToClient(
	tx *bbolt.Tx,
	stream bboltproto.DatasetService_SyncToServer,
	stats *bboltproto.SyncToComplete,
	datasetIDSet map[uuid.UUID]struct{},
	dsrBucket *bbolt.Bucket,
	datasetsBucket *bbolt.Bucket,
	commitsBucket *bbolt.Bucket,
	branchName string,
) error {
	bucket := tx.Bucket(keyNamedGraphRevisions)
	if bucket == nil {
		return nil
	}

	count := 0
	err := bucket.ForEach(func(k, v []byte) error {
		// Filter by datasetIDSet and/or branchName if provided
		if datasetIDSet != nil || !isAllBranches(branchName) {
			ngRevisionHash := BytesToHash(k)
			if !isNamedGraphRevisionForDatasets(ngRevisionHash, datasetIDSet, branchName, dsrBucket, datasetsBucket, commitsBucket) {
				return nil // Not for specified datasets/branch, skip
			}
		}

		resp := &bboltproto.SyncToResponse{
			Data: &bboltproto.SyncToResponse_BucketData{
				BucketData: &bboltproto.SyncToBucketData{
					BucketName: "ngr",
					Key:        k,
					Value:      v,
					IsBucket:   false,
				},
			},
		}
		if err := stream.Send(resp); err != nil {
			return err
		}
		count++
		return nil
	})
	stats.NamedGraphRevisionsSynced = int32(count)
	return err
}

func (s *datasetServiceServer) streamDatasetRevisionsToClient(
	tx *bbolt.Tx,
	stream bboltproto.DatasetService_SyncToServer,
	stats *bboltproto.SyncToComplete,
	datasetIDSet map[uuid.UUID]struct{},
	datasetsBucket *bbolt.Bucket,
	commitsBucket *bbolt.Bucket,
	branchName string,
) error {
	bucket := tx.Bucket(keyDatasetRevisions)
	if bucket == nil {
		return nil
	}

	count := 0
	err := bucket.ForEach(func(k, v []byte) error {
		subBucket := bucket.Bucket(k)
		if subBucket == nil {
			return nil
		}

		// Filter by datasetIDSet and/or branchName if provided
		if datasetIDSet != nil || !isAllBranches(branchName) {
			dsRevisionHash := BytesToHash(k)
			if !isDatasetRevisionForDatasets(dsRevisionHash, datasetIDSet, branchName, datasetsBucket, commitsBucket) {
				return nil // Not for specified datasets/branch, skip
			}
		}

		// Send sub-bucket entries
		return subBucket.ForEach(func(sk, sv []byte) error {
			resp := &bboltproto.SyncToResponse{
				Data: &bboltproto.SyncToResponse_BucketData{
					BucketData: &bboltproto.SyncToBucketData{
						BucketName: "dsr",
						Key:        k,
						Value:      nil, // Not used for sub-buckets
						IsBucket:   true,
						SubKey:     sk,
						SubValue:   sv,
					},
				},
			}
			if err := stream.Send(resp); err != nil {
				return err
			}
			count++
			return nil
		})
	})
	stats.DatasetRevisionsSynced = int32(count)
	return err
}

func (s *datasetServiceServer) streamDatasetsToClient(
	tx *bbolt.Tx,
	stream bboltproto.DatasetService_SyncToServer,
	stats *bboltproto.SyncToComplete,
	datasetIDSet map[uuid.UUID]struct{},
	branchName string,
) error {
	bucket := tx.Bucket(keyDatasets)
	if bucket == nil {
		return nil
	}

	datasetCount := 0
	err := bucket.ForEach(func(k, v []byte) error {
		subBucket := bucket.Bucket(k)
		if subBucket == nil {
			return nil
		}

		// Filter by datasetIDSet if provided
		if datasetIDSet != nil {
			dsID := uuid.UUID(k)
			if _, ok := datasetIDSet[dsID]; !ok {
				return nil // Dataset not in the set, skip
			}
		}

		// If branchName is specified, only send the specified branch entry
		if !isAllBranches(branchName) {
			branchKey := bytesToRefKey(stringAsImmutableBytes(branchName), refBranchPrefix)
			branchValue := subBucket.Get(branchKey)
			if branchValue == nil {
				// Branch not found for this dataset, skip
				return nil
			}
			// Send only the specified branch entry
			datasetCount++ // Count unique datasets
			resp := &bboltproto.SyncToResponse{
				Data: &bboltproto.SyncToResponse_BucketData{
					BucketData: &bboltproto.SyncToBucketData{
						BucketName: "ds",
						Key:        k,
						Value:      nil, // Not used for sub-buckets
						IsBucket:   true,
						SubKey:     branchKey,
						SubValue:   branchValue,
					},
				},
			}
			return stream.Send(resp)
		}

		datasetCount++ // Count unique datasets

		// Send sub-bucket entries
		return subBucket.ForEach(func(sk, sv []byte) error {
			resp := &bboltproto.SyncToResponse{
				Data: &bboltproto.SyncToResponse_BucketData{
					BucketData: &bboltproto.SyncToBucketData{
						BucketName: "ds",
						Key:        k,
						Value:      nil, // Not used for sub-buckets
						IsBucket:   true,
						SubKey:     sk,
						SubValue:   sv,
					},
				},
			}
			return stream.Send(resp)
		})
	})
	stats.DatasetsSynced = int32(datasetCount)
	return err
}

func (s *datasetServiceServer) streamCommitsToClient(
	tx *bbolt.Tx,
	stream bboltproto.DatasetService_SyncToServer,
	stats *bboltproto.SyncToComplete,
	datasetIDSet map[uuid.UUID]struct{},
	branchName string,
) error {
	bucket := tx.Bucket(keyCommits)
	if bucket == nil {
		return nil
	}

	// Get datasetsBucket for branch filtering
	var datasetsBucket *bbolt.Bucket
	if !isAllBranches(branchName) {
		datasetsBucket = tx.Bucket(keyDatasets)
	}

	count := 0
	err := bucket.ForEach(func(k, v []byte) error {
		subBucket := bucket.Bucket(k)
		if subBucket == nil {
			return nil
		}

		commitHash := BytesToHash(k)

		// Filter by branch if specified
		if !isAllBranches(branchName) && datasetsBucket != nil {
			if !isCommitInBranch(commitHash, branchName, datasetIDSet, datasetsBucket) {
				return nil // Not in specified branch, skip
			}
		}

		// Filter by datasetIDSet if provided
		if datasetIDSet != nil {
			if !isCommitForDatasets(commitHash, datasetIDSet, subBucket) {
				return nil // Not for specified datasets, skip
			}
		}

		// Send sub-bucket entries
		return subBucket.ForEach(func(sk, sv []byte) error {
			resp := &bboltproto.SyncToResponse{
				Data: &bboltproto.SyncToResponse_BucketData{
					BucketData: &bboltproto.SyncToBucketData{
						BucketName: "c",
						Key:        k,
						Value:      nil, // Not used for sub-buckets
						IsBucket:   true,
						SubKey:     sk,
						SubValue:   sv,
					},
				},
			}
			if err := stream.Send(resp); err != nil {
				return err
			}
			count++
			return nil
		})
	})
	stats.CommitsSynced = int32(count)
	return err
}

func (s *datasetServiceServer) streamDocumentInfoToClient(
	tx *bbolt.Tx,
	stream bboltproto.DatasetService_SyncToServer,
	stats *bboltproto.SyncToComplete,
	datasetIDSet map[uuid.UUID]struct{},
) error {
	// If datasetIDSet is provided, skip syncing documents
	if datasetIDSet != nil {
		return nil
	}

	bucket := tx.Bucket([]byte(documentInfoBucket))
	if bucket == nil {
		return nil
	}

	count := 0
	err := bucket.ForEach(func(k, v []byte) error {
		subBucket := bucket.Bucket(k)
		if subBucket == nil {
			return nil
		}

		// Send sub-bucket entries
		return subBucket.ForEach(func(sk, sv []byte) error {
			resp := &bboltproto.SyncToResponse{
				Data: &bboltproto.SyncToResponse_BucketData{
					BucketData: &bboltproto.SyncToBucketData{
						BucketName: "document_info",
						Key:        k,
						Value:      nil, // Not used for sub-buckets
						IsBucket:   true,
						SubKey:     sk,
						SubValue:   sv,
					},
				},
			}
			if err := stream.Send(resp); err != nil {
				return err
			}
			count++
			return nil
		})
	})
	stats.DocumentInfoSynced = int32(count)
	return err
}

// verifyDocumentFiles checks if document files exist in the vault for synced document_info
func (s *datasetServiceServer) verifyDocumentFiles(localRepo *localFullRepository, documentHashes map[string]bool) error {
	vaultDir := filepath.Join(localRepo.config.repositoryDir, VaultDirName)
	missingFiles := []string{}

	for hashStr := range documentHashes {
		hash, err := StringToHash(hashStr)
		if err != nil {
			continue // Skip invalid hash
		}

		fullPath := vaultPath(vaultDir, hash)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			missingFiles = append(missingFiles, hashStr)
		}
	}

	if len(missingFiles) > 0 {
		maxShow := 5
		if len(missingFiles) < maxShow {
			maxShow = len(missingFiles)
		}
		return fmt.Errorf("missing document files for %d documents: %v", len(missingFiles), missingFiles[:maxShow])
	}

	return nil
}

func (s *datasetServiceServer) processBucketData(
	db *bbolt.DB,
	bd *bboltproto.SyncFromBucketData,
	stats *bboltproto.SyncFromResponse,
	documentHashesToSync map[string]bool,
) error {
	bucketName := bd.BucketName
	key := bd.Key
	value := bd.Value

	return db.Update(func(tx *bbolt.Tx) error {
		var bucket *bbolt.Bucket
		var err error

		switch bucketName {
		case "ngr":
			bucket, err = tx.CreateBucketIfNotExists(keyNamedGraphRevisions)
			if err != nil {
				return err
			}
			if bucket.Get(key) == nil {
				if err := bucket.Put(key, value); err != nil {
					return err
				}
				stats.NamedGraphRevisionsSynced++
			}
		case "dsr":
			bucket, err = tx.CreateBucketIfNotExists(keyDatasetRevisions)
			if err != nil {
				return err
			}
			subBucket := bucket.Bucket(key)
			if subBucket == nil {
				subBucket, err = bucket.CreateBucket(key)
				if err != nil {
					return err
				}
				if bd.IsBucket && len(bd.SubKey) > 0 {
					if err := subBucket.Put(bd.SubKey, bd.SubValue); err != nil {
						return err
					}
				}
				stats.DatasetRevisionsSynced++
			} else if bd.IsBucket && len(bd.SubKey) > 0 {
				// Add sub-entry to existing bucket
				if subBucket.Get(bd.SubKey) == nil {
					if err := subBucket.Put(bd.SubKey, bd.SubValue); err != nil {
						return err
					}
				}
			}
		case "ds":
			bucket, err = tx.CreateBucketIfNotExists(keyDatasets)
			if err != nil {
				return err
			}
			dsBucket := bucket.Bucket(key)
			if dsBucket == nil {
				dsBucket, err = bucket.CreateBucket(key)
				if err != nil {
					return err
				}
				if bd.IsBucket && len(bd.SubKey) > 0 {
					if err := dsBucket.Put(bd.SubKey, bd.SubValue); err != nil {
						return err
					}
				}
				stats.DatasetsSynced++
			} else if bd.IsBucket && len(bd.SubKey) > 0 {
				// Check if identical or merge
				existingValue := dsBucket.Get(bd.SubKey)
				if !bytes.Equal(existingValue, bd.SubValue) {
					// TO BE SPECIFIED: For Datasets with different content, perform detailed analysis
					// of all revisions and all Named Branches and their parents.
					// Current implementation: simplified merge (keep existing if different)
					if existingValue == nil {
						if err := dsBucket.Put(bd.SubKey, bd.SubValue); err != nil {
							return err
						}
					}
					// If exists and different, keep existing (simplified merge)
				}
			}
		case "c":
			bucket, err = tx.CreateBucketIfNotExists(keyCommits)
			if err != nil {
				return err
			}
			commitBucket := bucket.Bucket(key)
			if commitBucket == nil {
				commitBucket, err = bucket.CreateBucket(key)
				if err != nil {
					return err
				}
				if bd.IsBucket && len(bd.SubKey) > 0 {
					if err := commitBucket.Put(bd.SubKey, bd.SubValue); err != nil {
						return err
					}
				}
				stats.CommitsSynced++
			} else if bd.IsBucket && len(bd.SubKey) > 0 {
				// Check if identical
				existingValue := commitBucket.Get(bd.SubKey)
				if existingValue == nil {
					if err := commitBucket.Put(bd.SubKey, bd.SubValue); err != nil {
						return err
					}
				} else if !bytes.Equal(existingValue, bd.SubValue) {
					// TO BE SPECIFIED: For Commits with different content, perform detailed comparison
					// of Named Branches.
					// Current implementation: keep existing (simplified)
				}
			}
		case "document_info":
			bucket, err = tx.CreateBucketIfNotExists([]byte(documentInfoBucket))
			if err != nil {
				return err
			}
			docBucket := bucket.Bucket(key)
			if docBucket == nil {
				docBucket, err = bucket.CreateBucket(key)
				if err != nil {
					return err
				}
				stats.DocumentInfoSynced++
			}
			documentHashesToSync[string(key)] = true
			if bd.IsBucket && len(bd.SubKey) > 0 {
				if docBucket.Get(bd.SubKey) == nil {
					if err := docBucket.Put(bd.SubKey, bd.SubValue); err != nil {
						return err
					}
				}
			}
		default:
			return fmt.Errorf("unknown bucket name: %s", bucketName)
		}

		return nil
	})
}

type suggestion interface {
	suggestion()
}

type suggestionImpl struct{}

func (suggestionImpl) suggestion() {}

type suggestDatasetWithCommitParents struct {
	suggestionImpl
	*bboltproto.DatasetSuggestion_SuggestDatasetWithCommitParents
}

type suggestDatasetNoCommitParents struct {
	suggestionImpl
	*bboltproto.DatasetSuggestion_SuggestDatasetNoCommitParents
}

func (s *datasetServiceServer) PushDatasets(stream bboltproto.DatasetService_PushDatasetsServer) error {
	log.Panic("gRPC datasetServiceServer received PushDatasets request, not expected to be used")
	return nil
}

type remoteDatasetPushStream struct {
	bboltproto.DatasetService_PushDatasetsServer
	suggestions chan suggestion
}

func (s remoteDatasetPushStream) Recv() (*bboltproto.DatasetObject, error) {
	for {
		m, err := s.DatasetService_PushDatasetsServer.Recv()
		if err != nil {
			return nil, err
		}
		switch sgg := m.Sgg.(type) {
		case *bboltproto.DatasetSuggestion_SuggestDatasetWithCommitParents:
			s.suggestions <- suggestDatasetWithCommitParents{DatasetSuggestion_SuggestDatasetWithCommitParents: sgg}
		case *bboltproto.DatasetSuggestion_SuggestDatasetNoCommitParents:
			s.suggestions <- suggestDatasetNoCommitParents{DatasetSuggestion_SuggestDatasetNoCommitParents: sgg}
		case *bboltproto.DatasetSuggestion_Obj:
			return sgg.Obj, nil
		}
	}
}

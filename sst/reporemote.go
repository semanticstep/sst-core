// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/semanticstep/sst-core/bboltproto"
	"github.com/semanticstep/sst-core/bleveproto"
	"github.com/semanticstep/sst-core/sstauth"
	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/mapping"
	index "github.com/blevesearch/bleve_index_api"
	"github.com/google/uuid"
	fs "github.com/relab/wrfs"
	"go.etcd.io/bbolt"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/oauth"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var emptyListRefsResponse = bboltproto.ListRefsResponse{}

// panicf panics with a formatted message that includes the file and line number.
func panicf(format string, args ...interface{}) {
	_, file, line, ok := runtime.Caller(1)
	msg := fmt.Sprintf(format, args...)
	if ok {
		panic(fmt.Sprintf("%s:%d: %s", filepath.Base(file), line, msg))
	}
	panic(msg)
}

// wrapError wraps an error with file and line number information.
func wrapError(err error) error {
	if err == nil {
		return nil
	}
	_, file, line, ok := runtime.Caller(1)
	if !ok {
		return err
	}
	return fmt.Errorf("%s:%d: %w", filepath.Base(file), line, err)
}

type dsIDCommitIDBrNameKey struct {
	dsID     uuid.UUID
	commitID Hash
	brName   string
}

var (
	ErrRepoClosed   = errors.New("repo is closed")
	ErrStageExpired = errors.New("stage lease expired (repo closing/closed)")
	ErrRepoNotFound = errors.New("repository not found")
)

type state int32

const (
	stateClosed state = iota
	stateOpen
	stateClosing
)

type stateControl struct {
	state atomic.Int32   // stateClosed / stateOpen / stateClosing
	wg    sync.WaitGroup // for waiting inflight stages close
}

type remoteRepository struct {
	stateControl
	url      *url.URL
	cc       *grpc.ClientConn // connection
	repoName string
	sr       SuperRepository

	dsClient       bboltproto.DatasetServiceClient // client
	commitClient   bboltproto.CommitServiceClient  // client
	reqCache       *remoteRequestCache
	remoteIndexIns remoteIndex
}

func (r *remoteRepository) SuperRepository() SuperRepository {
	return r.sr
}

func (r *remoteRepository) URL() string {
	return r.url.String()
}

// Unified gate to check if the repo is open and increment the waitGroup counter.
func (r *stateControl) enter() error {
	if state(r.state.Load()) != stateOpen {
		return ErrRepoClosed
	}
	r.wg.Add(1)
	// Double check, avoiding the minimal race window of Repo.Close()
	if state(r.state.Load()) != stateOpen {
		r.wg.Done()
		return ErrRepoClosed
	}
	return nil
}

// Decrement the waitGroup counter.
func (r *stateControl) leave() { r.wg.Done() }

func (r *remoteRepository) OpenStage(mode TriplexMode) Stage {
	if err := r.enter(); err != nil {
		GlobalLogger.Error("failed to open stage", zap.Error(err))
		return nil
	}
	defer r.leave()

	return &stage{
		repo:                     r,
		localGraphs:              map[uuid.UUID]*namedGraph{},
		referencedGraphs:         map[string]*namedGraph{},
		assignedNamedGraphNumber: 1,
	}
}

type remoteIndex struct {
	repoName string
	idx      bleveproto.IndexServiceClient
}

func (i *remoteIndex) Search(req *bleve.SearchRequest) (*bleve.SearchResult, error) {
	return i.SearchInContext(context.Background(), req)
}

func (i *remoteIndex) SearchInContext(ctx context.Context, req *bleve.SearchRequest) (*bleve.SearchResult, error) {
	sReqBytes, err := bleveproto.NewSearchRequest(req)
	if err != nil {
		GlobalLogger.Error("", zap.Error(err))
		return nil, err
	}

	var opts []grpc.CallOption
	authProvider := sstauth.AuthProviderFromContext(ctx)
	if p, ok := authProvider.(sstauth.Provider); ok {
		token, err := p.Oauth2Token()
		if err != nil {
			GlobalLogger.Error("", zap.Error(err))
			return nil, err
		}
		opts = append(opts, grpc.PerRPCCredentials(oauth.NewOauthAccess(token)))
	}

	GlobalLogger.Info("gRPC Clinet send search request", zap.String("repoName", i.repoName))
	// connectStart := time.Now()
	// fmt.Printf("***** Connect Start at %+v *****\n", connectStart)
	stream, err := i.idx.Search(ctx, opts...)
	if err != nil {
		GlobalLogger.Error("", zap.Error(err))
		return nil, err
	}
	// msg, msgFound := i.reqMessageCache.Get(string(sReqBytes))
	// if !msgFound {
	msg := &grpc.PreparedMsg{}
	err = msg.Encode(
		stream,
		&bleveproto.SearchRequest{
			Request:  sReqBytes,
			RepoName: i.repoName,
		},
	)
	if err != nil {
		GlobalLogger.Error("", zap.Error(err))
		return nil, err
	}
	// 	i.reqMessageCache.Set(string(sReqBytes), msg, 0)
	// }
	// sendStart := time.Now()
	// fmt.Printf("***** Send Start at %+v *****\n", sendStart)
	err = stream.SendMsg(msg)
	if err != nil {
		GlobalLogger.Error("", zap.Error(err))
		return nil, err
	}
	sRes, err := stream.CloseAndRecv()
	// receiveStart := time.Now()
	// fmt.Printf("***** Receive Start at %+v *****\n", receiveStart)
	if err != nil {
		GlobalLogger.Error("", zap.Error(err))
		return nil, err
	}
	return sRes.ToSearchResult()
}

var ErrNotImplemented = errors.New("not implemented")

func (*remoteIndex) DocCount() (uint64, error)                 { return 0, ErrNotImplemented }
func (*remoteIndex) Close() error                              { return nil }
func (*remoteIndex) Index(string, interface{}) error           { return ErrNotImplemented }
func (*remoteIndex) Delete(string) error                       { return ErrNotImplemented }
func (*remoteIndex) NewBatch() *bleve.Batch                    { return nil }
func (*remoteIndex) Batch(b *bleve.Batch) error                { return ErrNotImplemented }
func (*remoteIndex) Document(string) (index.Document, error)   { return nil, ErrNotImplemented }
func (*remoteIndex) Fields() ([]string, error)                 { return nil, ErrNotImplemented }
func (*remoteIndex) FieldDict(string) (index.FieldDict, error) { return nil, ErrNotImplemented }
func (*remoteIndex) Mapping() mapping.IndexMapping             { return nil }
func (*remoteIndex) Stats() *bleve.IndexStat                   { return nil }
func (*remoteIndex) StatsMap() map[string]interface{}          { return nil }
func (*remoteIndex) GetInternal([]byte) ([]byte, error)        { return nil, ErrNotImplemented }
func (*remoteIndex) SetInternal([]byte, []byte) error          { return ErrNotImplemented }
func (*remoteIndex) DeleteInternal(key []byte) error           { return ErrNotImplemented }
func (*remoteIndex) Name() string                              { return "" }
func (*remoteIndex) SetName(string)                            {}
func (*remoteIndex) Advanced() (index.Index, error)            { return nil, ErrNotImplemented }

func (*remoteIndex) FieldDictRange(string, []byte, []byte) (index.FieldDict, error) {
	return nil, ErrNotImplemented
}

func (*remoteIndex) FieldDictPrefix(string, []byte) (index.FieldDict, error) {
	return nil, ErrNotImplemented
}

const (
	maxStageCacheKeys = 256
	maxStageCacheTTL  = 30 * time.Second
)

func (r *remoteRepository) RegisterIndexHandler(*SSTDeriveInfo) error {
	return ErrNotAvailable
}

func (r *remoteRepository) Bleve() bleve.Index {
	if err := r.enter(); err != nil {
		GlobalLogger.Error("failed to open stage", zap.Error(err))
		return nil
	}
	defer r.leave()

	return &r.remoteIndexIns
}

// RebuildBleveIndex is not supported for remote repositories.
// Rebuilding the Bleve index is a server-side operation that must be
// performed on the host running the gRPC server.
func (r *remoteRepository) RebuildBleveIndex() error {
	return ErrNotSupported
}

// Use grpc.NewClient to create a new gRPC "channel" for the target URL provided. No I/O is performed.
func dialRepository(targetURL string, opts []grpc.DialOption) (*remoteRepository, error) {
	cc, err := grpc.NewClient(targetURL, opts...)
	if err != nil {
		GlobalLogger.Error("", zap.Error(err))
		return nil, err
	}

	returnedRepo := &remoteRepository{
		url:          &url.URL{Scheme: "", Host: targetURL},
		cc:           cc,
		dsClient:     bboltproto.NewDatasetServiceClient(cc),
		commitClient: bboltproto.NewCommitServiceClient(cc),
		reqCache:     newRemoteRequestCache(),
		remoteIndexIns: remoteIndex{
			idx: bleveproto.NewIndexServiceClient(cc),
		},
	}
	returnedRepo.state.Store(int32(stateOpen))

	return returnedRepo, nil
}

// DatasetIDs retrieves all dataset IDs from the remote repository.
// It makes repeated gRPC calls to the ListDatasets method of the dsClient
// until all pages of datasets are retrieved.
//
// Parameters:
// - ctx: The context for managing request deadlines and cancellations.
//
// Returns:
// - A slice of UUIDs representing the dataset IDs.
func (r *remoteRepository) DatasetIDs(ctx context.Context) ([]uuid.UUID, error) {
	if err := r.enter(); err != nil {
		GlobalLogger.Error("", zap.Error(err))
		return nil, err
	}
	defer r.leave()

	var datasets []uuid.UUID
	var datasetsPageToken string

	req := bboltproto.ListDatasetsRequest{
		PageToken: datasetsPageToken,
		PageSize:  1024,
		RepoName:  r.repoName,
	}
	req.MaskOut_Uuid()

	var opts []grpc.CallOption
	authProvider := sstauth.AuthProviderFromContext(ctx)
	if p, ok := authProvider.(sstauth.Provider); ok {
		token, err := p.Oauth2Token()
		if err != nil {
			GlobalLogger.Error("", zap.Error(err))
			return nil, err
		}
		opts = append(opts, grpc.PerRPCCredentials(oauth.NewOauthAccess(token)))
	}

	for {
		req.PageToken = datasetsPageToken
		GlobalLogger.Debug("gRPC dsClient call ListDatasets")
		resp, err := r.dsClient.ListDatasets(ctx, &req, opts...)
		if err != nil {
			GlobalLogger.Error("", zap.Error(err))
			return nil, err
		}
		for _, d := range resp.Datasets {
			datasetID, err := uuid.FromBytes(d.Uuid)
			if err != nil {
				GlobalLogger.Error("", zap.Error(err))
				return nil, err
			}
			datasets = append(datasets, datasetID)
		}
		if resp.NextPageToken == "" {
			break
		}
		datasetsPageToken = resp.NextPageToken
	}

	return datasets, nil
}

func (r *remoteRepository) Datasets(ctx context.Context) ([]IRI, error) {
	if err := r.enter(); err != nil {
		GlobalLogger.Error("", zap.Error(err))
		return nil, err
	}
	defer r.leave()

	var datasets []IRI
	var datasetsPageToken string

	req := bboltproto.ListDatasetsRequest{
		PageToken: datasetsPageToken,
		PageSize:  1024,
		RepoName:  r.repoName,
	}
	req.MaskOut_Uuid()
	req.MaskOut_Iri()

	var opts []grpc.CallOption
	authProvider := sstauth.AuthProviderFromContext(ctx)
	if p, ok := authProvider.(sstauth.Provider); ok {
		token, err := p.Oauth2Token()
		if err != nil {
			GlobalLogger.Error("", zap.Error(err))
			return nil, err
		}
		opts = append(opts, grpc.PerRPCCredentials(oauth.NewOauthAccess(token)))
	}

	for {
		req.PageToken = datasetsPageToken
		GlobalLogger.Debug("gRPC dsClient call ListDatasets")
		resp, err := r.dsClient.ListDatasets(ctx, &req, opts...)
		if err != nil {
			GlobalLogger.Error("", zap.Error(err))
			return nil, err
		}
		for _, d := range resp.Datasets {
			if d.Iri != nil {
				datasets = append(datasets, IRI(strings.TrimSuffix(*d.Iri, "#")))
			} else {
				GlobalLogger.Warn("Dataset IRI is missing, constructing IRI from UUID", zap.String("Dataset UUID", uuid.UUID(d.Uuid).String()))
				// If IRI is not available, construct it from the UUID
				id, err := uuid.FromBytes(d.Uuid)
				if err != nil {
					GlobalLogger.Error("", zap.Error(err))
					return nil, err
				}
				datasets = append(datasets, IRI(fmt.Sprintf("urn:uuid:%s#", id.String())))
			}
		}
		if resp.NextPageToken == "" {
			break
		}
		datasetsPageToken = resp.NextPageToken
	}

	return datasets, nil
}

func (r *remoteRepository) ForDatasets(ctx context.Context, c func(ds Dataset) error) error {
	if err := r.enter(); err != nil {
		GlobalLogger.Error("", zap.Error(err))
		return err
	}
	defer r.leave()

	var datasetsPageToken string

	req := bboltproto.ListDatasetsRequest{
		PageToken: datasetsPageToken,
		PageSize:  1024,
		RepoName:  r.repoName,
	}
	req.MaskOut_Uuid()
	req.MaskOut_Iri()

	var opts []grpc.CallOption
	authProvider := sstauth.AuthProviderFromContext(ctx)
	if p, ok := authProvider.(sstauth.Provider); ok {
		token, err := p.Oauth2Token()
		if err != nil {
			GlobalLogger.Error("", zap.Error(err))
			return err
		}
		opts = append(opts, grpc.PerRPCCredentials(oauth.NewOauthAccess(token)))
	}

	for {
		req.PageToken = datasetsPageToken
		GlobalLogger.Debug("gRPC dsClient call ListDatasets")
		resp, err := r.dsClient.ListDatasets(ctx, &req, opts...)
		if err != nil {
			GlobalLogger.Error("", zap.Error(err))
			return err
		}
		for _, d := range resp.Datasets {
			id, err := uuid.FromBytes(d.Uuid)
			if err != nil {
				GlobalLogger.Error("", zap.Error(err))
				continue
			}
			var iri IRI
			if d.Iri != nil {
				iri = IRI(strings.TrimSuffix(*d.Iri, "#"))
			}
			ds := &remoteDataset{r: r, refClient: bboltproto.NewRefServiceClient(r.cc), id: id, iri: iri}

			if err := c(ds); err != nil {
				return err
			}
		}
		if resp.NextPageToken == "" {
			break
		}
		datasetsPageToken = resp.NextPageToken
	}

	return nil
}

func (r *remoteRepository) datasetByID(ctx context.Context, id uuid.UUID) (Dataset, error) {
	if err := r.enter(); err != nil {
		GlobalLogger.Error("", zap.Error(err))
		return nil, err
	}
	defer r.leave()

	var opts []grpc.CallOption
	authProvider := sstauth.AuthProviderFromContext(ctx)
	if p, ok := authProvider.(sstauth.Provider); ok {
		token, err := p.Oauth2Token()
		if err != nil {
			GlobalLogger.Error("", zap.Error(err))
			return nil, err
		}
		opts = append(opts, grpc.PerRPCCredentials(oauth.NewOauthAccess(token)))
	}

	stream, err := r.dsClient.GetDataset(ctx, opts...)
	if err != nil {
		GlobalLogger.Error("", zap.Error(err))
		return nil, err
	}
	msg, found := r.reqCache.loadGetDatasetMsg(id)
	if !found {
		req := &bboltproto.GetDatasetRequest{
			DatasetId: &bboltproto.GetDatasetRequest_DatasetUuid{DatasetUuid: id[:]},
			ReadMask:  &fieldmaskpb.FieldMask{},
			RepoName:  r.repoName,
		}
		req.MaskOut_Iri()

		msg, err = r.reqCache.storeGetDatasetMsg(id, stream, req)
		if err != nil {
			GlobalLogger.Error("", zap.Error(err))
			return nil, err
		}
	}
	if err := stream.SendMsg(msg); err != nil {
		GlobalLogger.Error("", zap.Error(err))
		return nil, err
	}

	GlobalLogger.Debug("GetDataset request Sent", zap.String("Dataset UUID", id.String()))

	resp, err := stream.CloseAndRecv()
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Unknown {
			if s.Message() == ErrDatasetNotFound.Error() {
				err = ErrDatasetNotFound
				// ErrDatasetNotFound is an expected error, use Debug level
				GlobalLogger.Debug("Dataset not found", zap.String("Dataset UUID", id.String()))
				return nil, err
			}
		}
		GlobalLogger.Error("", zap.Error(err))
		return nil, err
	}

	// Extract IRI from response if available
	var iri IRI
	if resp != nil && resp.Iri != nil {
		iri = IRI(strings.TrimSuffix(*resp.Iri, "#"))
	}

	return &remoteDataset{r: r, refClient: bboltproto.NewRefServiceClient(r.cc), id: id, iri: iri}, nil
}

func (r *remoteRepository) Dataset(ctx context.Context, iri IRI) (Dataset, error) {
	id, err := iriToUUID(iri)
	if err != nil {
		return nil, err
	}
	return r.datasetByID(ctx, id)
}

func (r *remoteRepository) CommitDetails(ctx context.Context, ids []Hash) ([]*CommitDetails, error) {
	if err := r.enter(); err != nil {
		GlobalLogger.Error("", zap.Error(err))
		return nil, err
	}
	defer r.leave()

	commitClient := bboltproto.NewCommitServiceClient(r.cc)

	// Setup authentication if available
	var opts []grpc.CallOption
	if p, ok := sstauth.AuthProviderFromContext(ctx).(sstauth.Provider); ok {
		token, err := p.Oauth2Token()
		if err != nil {
			GlobalLogger.Error("", zap.Error(err))
			return nil, err
		}
		opts = append(opts, grpc.PerRPCCredentials(oauth.NewOauthAccess(token)))
	}

	// Prepare request
	var req bboltproto.GetCommitDetailsBatchRequest
	req.RepoName = r.repoName
	for _, id := range ids {
		req.CommitIds = append(req.CommitIds, id[:])
	}

	// Perform RPC call
	resp, err := commitClient.GetCommitDetailsBatch(ctx, &req, opts...)
	if err != nil {
		GlobalLogger.Error("", zap.Error(err))
		return nil, err
	}

	// Parse response into []*CommitDetails
	var results []*CommitDetails
	for _, d := range resp.Details {
		var id Hash
		copy(id[:], d.CommitId)

		// Convert DatasetRevisions (key is IRI string)
		dsRevs := make(map[IRI]Hash)
		for k, v := range d.DatasetRevisions {
			if len(v) != 32 {
				continue
			}
			var h Hash
			copy(h[:], v)
			dsRevs[IRI(k)] = h
		}

		// Convert NamedGraphRevisions (key is IRI string)
		ngRevs := make(map[IRI]Hash)
		for k, v := range d.NamedGraphRevisions {
			if len(v) != 32 {
				continue
			}
			var h Hash
			copy(h[:], v)
			ngRevs[IRI(k)] = h
		}

		// Convert ParentCommits (key is IRI string)
		parentCommits := make(map[IRI][]Hash)
		for k, v := range d.ParentCommits {
			var hashes []Hash
			for _, b := range v.Hashes {
				if len(b) != 32 {
					continue
				}
				var h Hash
				copy(h[:], b)
				hashes = append(hashes, h)
			}
			if len(hashes) > 0 {
				parentCommits[IRI(k)] = hashes
			}
		}

		results = append(results, &CommitDetails{
			Commit:              id,
			Author:              d.Author,
			Message:             d.Message,
			AuthorDate:          time.Unix(d.Timestamp, 0).UTC(),
			ParentCommits:       parentCommits,
			DatasetRevisions:    dsRevs,
			NamedGraphRevisions: ngRevs,
		})
	}

	return results, nil
}

func (r *remoteRepository) CheckoutCommit(ctx context.Context, commitID Hash, mode TriplexMode) (Stage, error) {
	if err := r.enter(); err != nil {
		return nil, err
	}
	defer r.leave()

	cds, err := r.CommitDetails(ctx, []Hash{commitID})
	if err != nil {
		return nil, err
	}
	if len(cds) == 0 || cds[0] == nil {
		return nil, ErrCommitNotFound
	}
	return checkoutCommitFromCommitDetails(ctx, r, commitID, mode, cds[0])
}

func (r *remoteRepository) Close() error {
	// mark stateClosing - CompareAndSwap will make sure only marked once
	r.state.CompareAndSwap(int32(stateOpen), int32(stateClosing))

	// Wait for all operations that have been entered to exit
	r.wg.Wait()

	// Close the gRPC connection
	if err := r.cc.Close(); err != nil {
		return err
	}

	r.state.Store(int32(stateClosed))

	return nil
}

type remoteDataset struct {
	r         *remoteRepository
	refClient bboltproto.RefServiceClient
	id        uuid.UUID
	iri       IRI
}

func (d *remoteDataset) FindCommonParentRevision(ctx context.Context, revision1, revision2 Hash) (parent Hash, err error) {
	if err := d.r.enter(); err != nil {
		return emptyHash, err
	}
	defer d.r.leave()

	req := bboltproto.FindCommonParentRevisionRequest{DatasetId: d.id[:], DatasetCommitRv1: revision1[:], DatasetCommitRv2: revision2[:], RepoName: d.r.repoName}
	GlobalLogger.Debug("gRPC dsClient call FindCommonParentRevision")
	var opts []grpc.CallOption
	authProvider := sstauth.AuthProviderFromContext(ctx)
	if p, ok := authProvider.(sstauth.Provider); ok {
		token, err := p.Oauth2Token()
		if err != nil {
			return HashNil(), err
		}
		opts = append(opts, grpc.PerRPCCredentials(oauth.NewOauthAccess(token)))
	}

	CommonParent, err := d.r.dsClient.FindCommonParentRevision(context.TODO(), &req, opts...)
	if err != nil {
		return HashNil(), err
	}

	return Hash(CommonParent.CommonParentCommitRv), err
}

func (d *remoteDataset) CommitForRevision(ctx context.Context, datasetRevision Hash) (Hash, error) {
	if err := d.r.enter(); err != nil {
		return HashNil(), err
	}
	defer d.r.leave()

	req := bboltproto.GetCommitForDatasetRevisionRequest{DatasetId: d.id[:], DatasetRevision: datasetRevision[:], RepoName: d.r.repoName}
	GlobalLogger.Debug("gRPC dsClient call GetCommitForDatasetRevision")
	var opts []grpc.CallOption
	authProvider := sstauth.AuthProviderFromContext(ctx)
	if p, ok := authProvider.(sstauth.Provider); ok {
		token, err := p.Oauth2Token()
		if err != nil {
			return HashNil(), err
		}
		opts = append(opts, grpc.PerRPCCredentials(oauth.NewOauthAccess(token)))
	}

	resp, err := d.r.dsClient.GetCommitForDatasetRevision(context.TODO(), &req, opts...)
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.NotFound {
			return HashNil(), ErrDatasetRevisionNotFound
		}
		return HashNil(), err
	}

	return BytesToHash(resp.CommitHash), nil
}

func (d *remoteDataset) IRI() IRI {
	if d.iri != "" {
		return d.iri
	}
	return IRI(fmt.Sprintf("urn:uuid:%s#", d.id.String()))
}

func (d *remoteDataset) Branches(ctx context.Context) (map[string]Hash, error) {
	if err := d.r.enter(); err != nil {
		GlobalLogger.Error("", zap.Error(err))
		return nil, err
	}
	defer d.r.leave()

	branchCommitHashMap := make(map[string]Hash)
	req := bboltproto.GetBranchesRequest{Uuid: d.id[:], RepoName: d.r.repoName}
	var opts []grpc.CallOption
	authProvider := sstauth.AuthProviderFromContext(ctx)
	if p, ok := authProvider.(sstauth.Provider); ok {
		token, err := p.Oauth2Token()
		if err != nil {
			GlobalLogger.Error("", zap.Error(err))
			return nil, err
		}
		opts = append(opts, grpc.PerRPCCredentials(oauth.NewOauthAccess(token)))
	}

	branchCommitHash, err := d.r.dsClient.GetBranches(context.TODO(), &req, opts...)
	if err != nil {
		return branchCommitHashMap, err
	}
	GlobalLogger.Debug(
		"Branches request Sent",
		zap.String("RepoName", req.RepoName),
		zap.String("Dataset UUID", uuid.UUID(req.Uuid).String()),
	)

	for _, val := range branchCommitHash.BranchRefs {
		branchCommitHashMap[val.Branch] = Hash(val.CommitHash)
	}

	return branchCommitHashMap, nil
}

func (d *remoteDataset) LeafCommits(ctx context.Context) ([]Hash, error) {
	if err := d.r.enter(); err != nil {
		return nil, wrapError(err)
	}
	defer d.r.leave()

	req := bboltproto.GetLeafCommitsRequest{Uuid: d.id[:], RepoName: d.r.repoName}

	var opts []grpc.CallOption
	authProvider := sstauth.AuthProviderFromContext(ctx)
	if p, ok := authProvider.(sstauth.Provider); ok {
		token, err := p.Oauth2Token()
		if err != nil {
			return nil, wrapError(err)
		}
		opts = append(opts, grpc.PerRPCCredentials(oauth.NewOauthAccess(token)))
	}

	commitHashes, err := d.r.dsClient.GetLeafCommits(context.TODO(), &req, opts...)
	if err != nil {
		return nil, wrapError(err)
	}

	GlobalLogger.Debug(
		"LeafCommits request Sent",
		zap.String("RepoName", req.RepoName),
		zap.String("Dataset UUID", uuid.UUID(req.Uuid).String()),
	)

	var returnedCommitHashes []Hash
	for _, value := range commitHashes.CommitHashes {
		returnedCommitHashes = append(returnedCommitHashes, BytesToHash(value))
	}

	return returnedCommitHashes, nil
}

func (d *remoteDataset) SetBranchCommit(ctx context.Context, commit Hash, branch string) error {
	if err := d.r.enter(); err != nil {
		return err
	}
	defer d.r.leave()

	req := bboltproto.SetBranchRequest{Uuid: d.id[:], Branch: branch, CommitHash: commit[:], RepoName: d.r.repoName}

	var opts []grpc.CallOption
	authProvider := sstauth.AuthProviderFromContext(ctx)
	if p, ok := authProvider.(sstauth.Provider); ok {
		token, err := p.Oauth2Token()
		if err != nil {
			return err
		}
		opts = append(opts, grpc.PerRPCCredentials(oauth.NewOauthAccess(token)))
	}

	_, err := d.r.dsClient.SetBranch(context.TODO(), &req, opts...)
	if err != nil {
		st, ok := status.FromError(err)
		if !ok {
			return wrapError(fmt.Errorf("failed to resolve gRPC error from SetBranch: %w", err))
		}
		return wrapError(errors.New(st.Message()))
	}

	GlobalLogger.Debug(
		"SetBranch request Sent",
		zap.String("RepoName", req.RepoName),
		zap.String("Branch", req.Branch),
		zap.String("Dataset UUID", uuid.UUID(req.Uuid).String()),
	)

	return nil
}

func (d *remoteDataset) SetBranchRevision(ctx context.Context, datasetRevision Hash, branch string) error {
	commit, err := d.CommitForRevision(ctx, datasetRevision)
	if err != nil {
		return err
	}
	return d.SetBranchCommit(ctx, commit, branch)
}

func (d *remoteDataset) RemoveBranch(ctx context.Context, branch string) error {
	if err := d.r.enter(); err != nil {
		return err
	}
	defer d.r.leave()

	req := bboltproto.RemoveBranchRequest{Uuid: d.id[:], Branch: branch, RepoName: d.r.repoName}

	var opts []grpc.CallOption
	authProvider := sstauth.AuthProviderFromContext(ctx)
	if p, ok := authProvider.(sstauth.Provider); ok {
		token, err := p.Oauth2Token()
		if err != nil {
			return err
		}
		opts = append(opts, grpc.PerRPCCredentials(oauth.NewOauthAccess(token)))
	}

	_, err := d.r.dsClient.RemoveBranch(context.TODO(), &req, opts...)
	if err != nil {
		st, ok := status.FromError(err)
		if !ok {
			return wrapError(fmt.Errorf("failed to resolve gRPC error from RemoveBranch: %w", err))
		}
		return wrapError(errors.New(st.Message()))
	}
	GlobalLogger.Debug(
		"RemoveBranch request sent",
		zap.String("RepoName", req.RepoName),
		zap.String("Branch", req.Branch),
		zap.String("Dataset UUID", uuid.UUID(req.Uuid).String()),
	)
	return nil
}

func (d *remoteDataset) ID() uuid.UUID {
	if err := d.r.enter(); err != nil {
		GlobalLogger.Error("failed to open stage", zap.Error(err))
		return uuid.UUID{}
	}
	defer d.r.leave()

	return d.id
}

func (d *remoteDataset) AssertStage(Stage) {}

func getAllImportsForRemoteRepository(ctx context.Context, stageDir fs.FS, ng NamedGraph, s Stage, alreadyLoaded map[uuid.UUID]struct{}) error {
	for _, di := range ng.DirectImports() {
		GlobalLogger.Debug("", zap.String("imported NG ID", di.ID().String()), zap.String("for NG with ID", ng.ID().String()))
		var dgFile fs.File
		dgFile, err := stageDir.Open(di.ID().String())
		if err != nil {
			return wrapError(fmt.Errorf("failed to open import file for %s: %w", di.ID().String(), err))
		}
		defer dgFile.Close()

		tempNg, err := SstRead(bufio.NewReader(dgFile), DefaultTriplexMode)
		if err != nil {
			return wrapError(fmt.Errorf("failed to read SST file for import %s: %w", di.ID().String(), err))
		}

		// merge temp info into main
		if _, found := alreadyLoaded[di.ID()]; !found {
			_, err = s.MoveAndMerge(ctx, tempNg.Stage())
			if err != nil {
				return wrapError(fmt.Errorf("failed to merge stage for import %s: %w", di.ID().String(), err))
			}
			alreadyLoaded[di.ID()] = struct{}{}
		}

		// run this recursively
		if err := getAllImportsForRemoteRepository(ctx, stageDir, s.NamedGraph(di.IRI()), s, alreadyLoaded); err != nil {
			return err
		}
	}
	return nil
}

func (d *remoteDataset) Repository() Repository {
	if err := d.r.enter(); err != nil {
		GlobalLogger.Error("failed to get Repository", zap.Error(err))
		return nil
	}
	defer d.r.leave()

	return d.r
}

func (d *remoteDataset) CheckoutCommit(ctx context.Context, commitID Hash, mode TriplexMode) (Stage, error) {
	if err := d.r.enter(); err != nil {
		GlobalLogger.Error("", zap.Error(err))
		return nil, err
	}
	defer d.r.leave()

	var st *stage

	var opts []grpc.CallOption
	authProvider := sstauth.AuthProviderFromContext(ctx)
	if p, ok := authProvider.(sstauth.Provider); ok {
		token, err := p.Oauth2Token()
		if err != nil {
			GlobalLogger.Error("", zap.Error(err))
			return nil, err
		}
		opts = append(opts, grpc.PerRPCCredentials(oauth.NewOauthAccess(token)))
	}

	GlobalLogger.Debug("gRPC dsClient call FetchDatasets")
	fetchStream, err := d.r.dsClient.FetchDatasets(ctx, opts...)
	if err != nil {
		return st, err
	}
	recvResult := make(chan error, 1)
	r := remoteCheckoutState{
		datasetRevisions:      map[Hash]*bboltproto.DatasetRevision{},
		graphContents:         map[Hash][]byte{},
		datasetToDefaultGraph: map[uuid.UUID]Hash{},
		revisionToCommit:      map[Hash]Hash{},
		datasetToRevision:     map[uuid.UUID]Hash{},
	}
	wantCommitID := &bboltproto.RefName_Leaf{Leaf: commitID[:]}
	go func() { recvResult <- r.recvStage(fetchStream, wantCommitID, nil) }()
	err = remoteStageFetchSend(fetchStream, d, wantCommitID,
		func() (*grpc.PreparedMsg, bool) {
			return d.r.reqCache.loadDatasetInquiryNoParentsByCommitMsg(d.id, commitID)
		}, func(req *bboltproto.DatasetInquiry) (*grpc.PreparedMsg, error) {
			return d.r.reqCache.storeDatasetInquiryNoParentsByCommitMsg(d.id, commitID, fetchStream, req)
		})
	recvErr := <-recvResult
	if err != nil {
		return st, err
	}
	if recvErr != nil {
		return st, recvErr
	}
	err = r.collectNamedGraphs(d.id)
	if err != nil {
		return st, err
	}

	stageFS := remoteRepoFsOf(r.graphContents, r.datasetToDefaultGraph)
	dgFile, err := stageFS.Open(d.id.String())
	if err != nil {
		return nil, wrapError(fmt.Errorf("failed to open dataset file for %s: %w", d.id.String(), err))
	}
	defer dgFile.Close()
	// read current Dataset's main NG and referenced NG into stage
	ng, err := SstRead(bufio.NewReader(dgFile), DefaultTriplexMode)
	if err != nil {
		return nil, wrapError(fmt.Errorf("failed to read SST file for dataset %s: %w", d.id.String(), err))
	}

	st = ng.Stage().(*stage)
	st.repo = d.r

	alreadyLoaded := make(map[uuid.UUID]struct{})

	if err = getAllImportsForRemoteRepository(ctx, stageFS, ng, st, alreadyLoaded); err != nil {
		return nil, err
	}

	// Populate checkout info using dsIDToRevision for direct lookup
	for _, ng := range st.localGraphs {
		ng.checkedOutNGRevisions = []Hash{r.datasetToDefaultGraph[ng.id]}
		if dsRevHash, ok := r.datasetToRevision[ng.id]; ok {
			ng.checkedOutDSRevisions = []Hash{dsRevHash}
			if commitHash, ok := r.revisionToCommit[dsRevHash]; ok {
				ng.checkedOutCommits = []Hash{commitHash}
			}
		}
	}

	// correct modified flag to false due to just checkout
	for _, ng := range st.localGraphs {
		ng.flags.modified = false
	}

	return st, nil
}

func (d *remoteDataset) CheckoutBranch(ctx context.Context, br string, mode TriplexMode) (Stage, error) {
	if err := d.r.enter(); err != nil {
		GlobalLogger.Error("", zap.Error(err))
		return nil, err
	}
	defer d.r.leave()

	commitHash, err := branchToCommitRemote(ctx, d, br)
	if err != nil {
		GlobalLogger.Error("", zap.Error(err))
		return nil, err
	}

	return d.CheckoutCommit(ctx, commitHash, DefaultTriplexMode)
}

func (d *remoteDataset) CheckoutRevision(ctx context.Context, datasetRevision Hash, mode TriplexMode) (Stage, error) {
	if err := d.r.enter(); err != nil {
		GlobalLogger.Error("", zap.Error(err))
		return nil, err
	}
	defer d.r.leave()

	var st *stage

	var opts []grpc.CallOption
	authProvider := sstauth.AuthProviderFromContext(ctx)
	if p, ok := authProvider.(sstauth.Provider); ok {
		token, err := p.Oauth2Token()
		if err != nil {
			GlobalLogger.Error("", zap.Error(err))
			return nil, err
		}
		opts = append(opts, grpc.PerRPCCredentials(oauth.NewOauthAccess(token)))
	}

	GlobalLogger.Debug("gRPC dsClient call FetchDatasets for CheckoutRevision")
	fetchStream, err := d.r.dsClient.FetchDatasets(ctx, opts...)
	if err != nil {
		return nil, err
	}

	// Send inquiry and close the client send side so the server knows no
	// further inquiries are coming. Without this CloseSend the server keeps
	// waiting for more messages and the stream deadlocks.
	err = fetchStream.Send(&bboltproto.DatasetInquiry{
		Inq: &bboltproto.DatasetInquiry_WantDatasetRevision{
			WantDatasetRevision: datasetRevision[:],
		},
		RepoName: d.r.repoName,
	})
	if err != nil {
		return nil, err
	}
	if err := fetchStream.CloseSend(); err != nil {
		return nil, err
	}

	recvResult := make(chan error, 1)
	r := remoteCheckoutState{
		datasetRevisions:      map[Hash]*bboltproto.DatasetRevision{},
		graphContents:         map[Hash][]byte{},
		datasetToDefaultGraph: map[uuid.UUID]Hash{},
		revisionToCommit:      map[Hash]Hash{},
		datasetToRevision:     map[uuid.UUID]Hash{},
	}
	go func() { recvResult <- r.recvStageRevision(fetchStream, datasetRevision) }()

	recvErr := <-recvResult
	if recvErr != nil {
		return nil, recvErr
	}

	if err = r.collectNamedGraphsForRevision(d.id, datasetRevision); err != nil {
		return nil, err
	}

	// Build stageFS and load named graphs
	stageFS := remoteRepoFsOf(r.graphContents, r.datasetToDefaultGraph)
	dgFile, err := stageFS.Open(d.id.String())
	if err != nil {
		return nil, wrapError(fmt.Errorf("failed to open dataset file for %s: %w", d.id.String(), err))
	}
	defer dgFile.Close()
	ng, err := SstRead(bufio.NewReader(dgFile), DefaultTriplexMode)
	if err != nil {
		return nil, wrapError(fmt.Errorf("failed to read SST file for dataset %s: %w", d.id.String(), err))
	}

	st = ng.Stage().(*stage)
	st.repo = d.r

	alreadyLoaded := make(map[uuid.UUID]struct{})
	if err = getAllImportsForRemoteRepository(ctx, stageFS, ng, st, alreadyLoaded); err != nil {
		return nil, err
	}

	// Populate checkout info
	for _, ng := range st.localGraphs {
		ng.checkedOutNGRevisions = []Hash{r.datasetToDefaultGraph[ng.id]}
		// Get the dataset revision hash for this dataset ID
		if dsRevHash, ok := r.datasetToRevision[ng.id]; ok {
			ng.checkedOutDSRevisions = []Hash{dsRevHash}
			if commitHash, ok := r.revisionToCommit[dsRevHash]; ok {
				ng.checkedOutCommits = []Hash{commitHash}
			}
		}
	}

	for _, ng := range st.localGraphs {
		ng.flags.modified = false
	}

	return st, nil
}

func (r *remoteCheckoutState) recvStageRevision(
	fetchStream bboltproto.DatasetService_FetchDatasetsClient, wantDSRevision Hash,
) error {
	return r.recvStageWithHandler(fetchStream, func(recv *bboltproto.DatasetObject) (done bool) {
		switch obj := recv.Obj.(type) {
		case *bboltproto.DatasetObject_NamedGraphRevision:
			ngHash := BytesToHash(obj.NamedGraphRevision.Hash)
			r.graphContents[ngHash] = obj.NamedGraphRevision.Content
		case *bboltproto.DatasetObject_DatasetRevision:
			dsHash := BytesToHash(obj.DatasetRevision.Hash)
			r.datasetRevisions[dsHash] = obj.DatasetRevision
		case *bboltproto.DatasetObject_DatasetRevisionCommitHash:
			ngBC := obj.DatasetRevisionCommitHash
			bCommitID := BytesToHash(ngBC.CommitHash)
			r.revisionToCommit[BytesToHash(ngBC.DatasetRevision)] = bCommitID
		case *bboltproto.DatasetObject_WantedDatasetRevision:
			if BytesToHash(obj.WantedDatasetRevision) == wantDSRevision {
				return true
			}
		}
		return false
	})
}

func (r *remoteCheckoutState) collectNamedGraphsForRevision(dsID uuid.UUID, rootDSHash Hash) error {
	if rootDSHash.IsNil() {
		r.datasetToDefaultGraph[dsID] = HashNil()
		return nil
	}
	return r.collectNamedGraphsFromRevision(dsID, rootDSHash)
}

func (r *remoteCheckoutState) collectNamedGraphsFromRevision(dsID uuid.UUID, dsHash Hash) error {
	type ngRevT struct {
		id   uuid.UUID
		hash Hash
	}
	for queue := []ngRevT{{id: dsID, hash: dsHash}}; len(queue) > 0; queue = queue[1:] {
		h := queue[0]
		dsRev := r.datasetRevisions[h.hash]
		if dsRev == nil {
			continue
		}
		// Track the dataset ID to revision mapping
		r.datasetToRevision[h.id] = h.hash
		defaultGraphHash := BytesToHash(dsRev.DefaultNamedGraphHash)
		r.datasetToDefaultGraph[h.id] = defaultGraphHash
		for _, dsImport := range dsRev.ImportedDatasets {
			dsImpID, err := uuid.FromBytes(dsImport.DatasetUuid)
			if err != nil {
				return err
			}
			if _, ok := r.datasetToDefaultGraph[dsImpID]; !ok {
				dsImpHash := BytesToHash(dsImport.DatasetHash)
				queue = append(queue, ngRevT{id: dsImpID, hash: dsImpHash})
			}
		}
	}
	return nil
}

// getCommit id based on branch Name for remoteDataset.
func branchToCommitRemote(ctx context.Context, ds *remoteDataset, br string) (Hash, error) {
	var err error
	var commitRev Hash
	var stream bboltproto.RefService_GetRefClient

	var opts []grpc.CallOption
	authProvider := sstauth.AuthProviderFromContext(ctx)
	if p, ok := authProvider.(sstauth.Provider); ok {
		token, err := p.Oauth2Token()
		if err != nil {
			return HashNil(), err
		}
		opts = append(opts, grpc.PerRPCCredentials(oauth.NewOauthAccess(token)))
	}

	if stream, err = ds.refClient.GetRef(ctx, opts...); err != nil {
		return HashNil(), err
	}
	msg, found := ds.r.reqCache.loadRefByBranchMsg(ds.id, br)
	if !found {
		msg, err = ds.r.reqCache.storeRefByBranchMsg(ds.id, br, stream, &bboltproto.GetRefRequest{
			RefName: &bboltproto.RefName{
				DatasetId: &bboltproto.RefName_DatasetUuid{DatasetUuid: ds.id[:]},
				RefName:   &bboltproto.RefName_Branch{Branch: br},
			},
			RepoName: ds.r.repoName,
		})
		if err != nil {
			return HashNil(), err
		}
	}
	if err = stream.SendMsg(msg); err != nil {
		return HashNil(), err
	}
	var resp *bboltproto.Ref
	if resp, err = stream.CloseAndRecv(); err != nil {
		return HashNil(), err
	}
	commitRev = BytesToHash(resp.CommitId)

	return commitRev, nil
}

func (d *remoteDataset) CommitDetailsByHash(
	ctx context.Context,
	commitID Hash,
) (*CommitDetails, error) {
	if err := d.r.enter(); err != nil {
		GlobalLogger.Error("", zap.Error(err))
		return nil, err
	}
	defer d.r.leave()

	cd, err := d.r.CommitDetails(ctx, []Hash{commitID})
	if err != nil {
		GlobalLogger.Error("", zap.Error(err))
		return nil, err
	}

	if len(cd) == 0 {
		return nil, wrapError(ErrCommitNotFound)
	}

	return cd[0], nil
}

func (d *remoteDataset) CommitDetailsByBranch(
	ctx context.Context,
	br string,
) (*CommitDetails, error) {
	if err := d.r.enter(); err != nil {
		GlobalLogger.Error("", zap.Error(err))
		return nil, err
	}
	defer d.r.leave()

	commitID, err := branchToCommitRemote(ctx, d, br)
	if err != nil {
		return nil, wrapError(fmt.Errorf("failed to get commit for branch %q: %w", br, err))
	}

	return d.CommitDetailsByHash(ctx, commitID)
}

// remoteCheckoutState accumulates data received from the remote repository during
// a checkout operation (by commit or dataset revision). It stores the necessary
// information to reconstruct a stage with all its named graphs and imports.
type remoteCheckoutState struct {
	// datasetRevisions maps dataset revision hashes to their proto objects.
	// Populated from DatasetObject_DatasetRevision messages.
	datasetRevisions map[Hash]*bboltproto.DatasetRevision

	// graphContents maps named graph revision hashes to their raw SST content.
	// Populated from DatasetObject_NamedGraphRevision messages.
	graphContents map[Hash][]byte

	// datasetToDefaultGraph maps dataset IDs to their default named graph hashes.
	// Used to locate a dataset's main graph within graphContents.
	datasetToDefaultGraph map[uuid.UUID]Hash

	// revisionToCommit maps dataset revision hashes to their associated commit hashes.
	// Populated from DatasetObject_DatasetRevisionCommitHash messages.
	// Used to set checkedOutCommits on named graphs.
	revisionToCommit map[Hash]Hash

	// targetCommit is the base commit being checked out (for commit-based checkout).
	// Populated from DatasetObject_Commit messages. May be nil for revision checkout.
	targetCommit *bboltproto.Commit

	// datasetToRevision maps dataset IDs to their dataset revision hashes.
	// Built during collectNamedGraphsFromRevision by traversing the import tree.
	// Used to set checkedOutDSRevisions on named graphs.
	datasetToRevision map[uuid.UUID]Hash
}

func (r *remoteRepository) commitNewVersion(ctx context.Context, st *stage, message string, branch string) (Hash, []uuid.UUID, error) {
	if err := r.enter(); err != nil {
		return emptyHash, nil, err
	}
	defer r.leave()
	GlobalLogger.Info("Committing to remote repository", zap.String("RepoName", r.repoName), zap.String("Branch", branch))

	var baseCommitID Hash
	ngRevisions := map[Hash][]byte{}

	if len(st.NamedGraphs()) > 0 {
		err := st.WriteSstFilesDirectory(remoteRepoFsOf(ngRevisions, nil))
		if err != nil {
			return baseCommitID, nil, err
		}
	}
	namedGraphs := make([][]byte, 0, len(ngRevisions))
	for _, content := range ngRevisions {
		namedGraphs = append(namedGraphs, content)
	}

	modifiedNGs := make([][]byte, 0)

	for ngUUID, ng := range st.localGraphs {
		if ng.flags.modified {
			uuidBytes := make([]byte, len(ngUUID))
			copy(uuidBytes, ngUUID[:])
			modifiedNGs = append(modifiedNGs, uuidBytes)
		}
	}

	// send NG's checkoutNamedGraphSignatures and checkoutDsIDAndHashes
	checkoutNamedGraphSignatures := make([]*bboltproto.NamedGraphSignature, 0, len(modifiedNGs))
	checkoutDatasetSignatures := make([]*bboltproto.DatasetSignature, 0, len(modifiedNGs))
	parentCommitOverrides := make([]*bboltproto.ParentCommit, 0, len(modifiedNGs))

	for _, ng := range st.localGraphs {
		lastNGRev := ng.lastCheckedOutNGRevision()
		lastDSRev := ng.lastCheckedOutDSRevision()
		checkoutNamedGraphSignatures = append(checkoutNamedGraphSignatures, &bboltproto.NamedGraphSignature{
			NgUuid: ng.id[:],
			NgHash: lastNGRev[:],
		})
		checkoutDatasetSignatures = append(checkoutDatasetSignatures, &bboltproto.DatasetSignature{
			DSUuid: ng.id[:],
			DSHash: lastDSRev[:],
		})

		var tempCheckoutCommits []byte
		for _, val := range ng.checkedOutCommits {
			tempCheckoutCommits = append(tempCheckoutCommits, val[:]...)
		}

		parentCommitOverrides = append(parentCommitOverrides, &bboltproto.ParentCommit{
			DatasetUuid: ng.id[:],
			CommitId:    tempCheckoutCommits,
		})
	}

	var baseCommitIDBytes []byte
	if baseCommitID != HashNil() {
		baseCommitIDBytes = baseCommitID[:]
	}
	req := bboltproto.CreateCommitRequest{
		RefName: &bboltproto.RefName{
			// DatasetId: &bboltproto.RefName_DatasetUuid{DatasetUuid: dsID[:]},
			DatasetId: &bboltproto.RefName_DatasetUuid{DatasetUuid: uuid.Nil[:]},
		},
		BaseCommit:                   baseCommitIDBytes,
		NamedGraphs:                  namedGraphs,
		ParentCommitOverrides:        parentCommitOverrides,
		Message:                      message,
		ModifiedNGs:                  modifiedNGs,
		CheckoutNamedGraphSignatures: checkoutNamedGraphSignatures,
		CheckoutDatasetSignatures:    checkoutDatasetSignatures,
		RepoName:                     r.repoName,
	}

	req.RefName.RefName = &bboltproto.RefName_Branch{Branch: branch}

	// if s.d.ephemeral {
	// 	req.Flags = append(req.Flags, bboltproto.CreateCommitRequest_skipRootDataset)
	// }
	// if (st.stageFlags() & ForbidDatasetBranchDivergence) == ForbidDatasetBranchDivergence {
	// 	req.Flags = append(req.Flags, bboltproto.CreateCommitRequest_forbidDatasetBranchDivergence)
	// }
	// if (st.stageFlags() & RepoCreateFirstLevelDatasetsOnly) == RepoCreateFirstLevelDatasetsOnly {
	// 	req.Flags = append(req.Flags, bboltproto.CreateCommitRequest_createFirstLevelDatasetsOnly)
	// }

	// fmt.Printf("client call CreateCommit")

	// resp, err := r.commitClient.CreateCommit(r.ctx, &req, s.d.callOptions...)

	var opts []grpc.CallOption
	authProvider := sstauth.AuthProviderFromContext(ctx)
	if p, ok := authProvider.(sstauth.Provider); ok {
		token, err := p.Oauth2Token()
		if err != nil {
			return HashNil(), nil, err
		}
		opts = append(opts, grpc.PerRPCCredentials(oauth.NewOauthAccess(token)))
	}

	resp, err := r.commitClient.CreateCommit(ctx, &req, opts...)
	if err != nil {
		if s, ok := status.FromError(err); ok {
			switch s.Code() {
			case codes.FailedPrecondition:
				err = fmt.Errorf("%w: %s", ErrPreCommitConditionFailed, s.Message())
			case codes.ResourceExhausted:
				err = fmt.Errorf("%w: %s", ErrQuotaExceeded, s.Message())
			case codes.Unknown:
				msg := s.Message()
				// if msg == DatasetBranchHasDiverged.Error() {
				// 	err = DatasetBranchHasDiverged
				// } else
				if msg == ErrCommitWasNotCreated.Error() {
					err = ErrCommitWasNotCreated
				}
				// } else if msg == ErrDatasetBranchWouldDiverge.Error() {
				// 	err = ErrDatasetBranchWouldDiverge
				// }
			case codes.OK, codes.Canceled, codes.InvalidArgument, codes.DeadlineExceeded, codes.NotFound,
				codes.AlreadyExists, codes.PermissionDenied, codes.Aborted, codes.OutOfRange,
				codes.Unimplemented, codes.Internal, codes.Unavailable, codes.DataLoss, codes.Unauthenticated:
			}
		}
		return baseCommitID, nil, err
	}

	GlobalLogger.Debug("CreateCommit request Sent")

	modifiedDatasets := make([]uuid.UUID, 0)

	// after commit, get generated NGHash from GRPC Server, update the NG checkedOutNGHash
	for _, NamedGraphSignature := range resp.CheckoutNamedGraphSignatures {
		if ng, ok := st.localGraphs[uuid.UUID(NamedGraphSignature.NgUuid)]; ok {
			ng.checkedOutNGRevisions = []Hash{Hash(NamedGraphSignature.NgHash)}
		}
	}

	for _, DatasetSignature := range resp.CheckoutDatasetSignatures {
		if ng, ok := st.localGraphs[uuid.UUID(DatasetSignature.DSUuid)]; ok {
			ng.checkedOutDSRevisions = []Hash{Hash(DatasetSignature.DSHash)}
			ng.checkedOutCommits = []Hash{BytesToHash(resp.CommitId)}
		}
	}

	for _, ID := range resp.ModifiedDatasets {
		var uuidVal uuid.UUID
		copy(uuidVal[:], ID)
		modifiedDatasets = append(modifiedDatasets, uuidVal)
	}

	generatedCommitHash := BytesToHash(resp.CommitId)
	return generatedCommitHash, modifiedDatasets, err
}

type (
	loadRemoteDatasetInqMsg   func() (*grpc.PreparedMsg, bool)
	storedRemoteDatasetInqMsg func(req *bboltproto.DatasetInquiry) (*grpc.PreparedMsg, error)
)

func (r *remoteCheckoutState) recvStage(
	fetchStream bboltproto.DatasetService_FetchDatasetsClient, wantRef bboltproto.BareRefName, recvCommitID *Hash,
) error {
	return r.recvStageWithHandler(fetchStream, func(recv *bboltproto.DatasetObject) (done bool) {
		switch obj := recv.Obj.(type) {
		case *bboltproto.DatasetObject_NamedGraphRevision:
			ngHash := BytesToHash(obj.NamedGraphRevision.Hash)
			r.graphContents[ngHash] = obj.NamedGraphRevision.Content
		case *bboltproto.DatasetObject_DatasetRevision:
			dsHash := BytesToHash(obj.DatasetRevision.Hash)
			r.datasetRevisions[dsHash] = obj.DatasetRevision
		case *bboltproto.DatasetObject_Commit:
			r.targetCommit = obj.Commit
		case *bboltproto.DatasetObject_DatasetRevisionCommitHash:
			ngBC := obj.DatasetRevisionCommitHash
			bCommitID := BytesToHash(ngBC.CommitHash)
			r.revisionToCommit[BytesToHash(ngBC.DatasetRevision)] = bCommitID
		case *bboltproto.DatasetObject_WantedDatasetNoCommitParents:
			switch refName := wantRef.(type) {
			case *bboltproto.RefName_Branch:
				if bytes.Equal([]byte(obj.WantedDatasetNoCommitParents.Ref.GetBranch()), []byte(refName.Branch)) {
					if recvCommitID != nil {
						copy(recvCommitID[:], obj.WantedDatasetNoCommitParents.CommitId)
					}
					return true
				}
			case *bboltproto.RefName_Leaf:
				if bytes.Equal(obj.WantedDatasetNoCommitParents.Ref.GetLeaf(), refName.Leaf) {
					if recvCommitID != nil {
						copy(recvCommitID[:], obj.WantedDatasetNoCommitParents.CommitId)
					}
					return true
				}
			}
		}
		return false
	})
}

func (r *remoteCheckoutState) recvStageWithHandler(
	fetchStream bboltproto.DatasetService_FetchDatasetsClient, handler func(*bboltproto.DatasetObject) bool,
) error {
	for {
		recv, err := fetchStream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			if status.Code(err) == codes.Canceled {
				return nil
			}
			return err
		}
		if handler(recv) {
			return nil
		}
	}
}

func remoteStageFetchSend(
	f bboltproto.DatasetService_FetchDatasetsClient,
	d *remoteDataset,
	wantRef bboltproto.BareRefName,
	loadMsg loadRemoteDatasetInqMsg,
	storeMsg storedRemoteDatasetInqMsg,
) (err error) {
	defer func() { _ = f.CloseSend() }()
	msg, found := loadMsg()
	if !found {
		msg, err = storeMsg(&bboltproto.DatasetInquiry{
			Inq: &bboltproto.DatasetInquiry_WantDatasetNoCommitParents{
				WantDatasetNoCommitParents: &bboltproto.RefName{
					DatasetId: &bboltproto.RefName_DatasetUuid{DatasetUuid: d.id[:]},
					RefName:   wantRef,
				},
			},
			RepoName: d.r.repoName,
		})
		if err != nil {
			return
		}
	}
	return f.SendMsg(msg)
}

func (r *remoteCheckoutState) collectNamedGraphs(dsID uuid.UUID) error {
	if r.targetCommit == nil {
		r.datasetToDefaultGraph[dsID] = HashNil()
		return nil
	}
	dsHash := HashNil()
	for _, dsImport := range r.targetCommit.Datasets {
		if bytes.Equal(dsImport.DatasetUuid, dsID[:]) {
			h := BytesToHash(dsImport.DatasetHash)
			dsHash = h
			break
		}
	}
	if dsHash.IsNil() {
		return nil
	}
	return r.collectNamedGraphsFromRevision(dsID, dsHash)
}

func implAsRemoteRepository(r Repository) (*remoteRepository, bool) {
	impl, ok := r.(*remoteRepository)
	return impl, ok
}

func (r *remoteRepository) Info(ctx context.Context, branchName string) (RepositoryInfo, error) {
	if err := r.enter(); err != nil {
		return RepositoryInfo{}, err
	}
	defer r.leave()

	GlobalLogger.Debug("gRPC dsClient call GetRepositoryInfo")
	var opts []grpc.CallOption
	authProvider := sstauth.AuthProviderFromContext(ctx)
	if p, ok := authProvider.(sstauth.Provider); ok {
		token, err := p.Oauth2Token()
		if err != nil {
			return RepositoryInfo{}, err
		}
		opts = append(opts, grpc.PerRPCCredentials(oauth.NewOauthAccess(token)))
	}

	GlobalLogger.Debug("Getting repository info", zap.String("RepoName", r.repoName), zap.String("BranchName", branchName))
	resp, err := r.dsClient.GetRepositoryInfo(ctx,
		&bboltproto.GetRepositoryInfoRequest{
			BranchName: branchName,
			RepoName:   r.repoName,
		}, opts...)
	if err != nil {
		return RepositoryInfo{}, fmt.Errorf("failed to get repository info: %w", err)
	}

	bleveInfo, err := r.dsClient.GetBleveInfo(ctx, &bboltproto.GetRepoBleveInfoRequest{RepoName: r.repoName}, opts...)
	if err != nil {
		return RepositoryInfo{}, wrapError(fmt.Errorf("failed to get Bleve info: %w", err))
	}

	return RepositoryInfo{
		URL:                         r.url.String(),
		AccessRight:                 sstauth.ParseAccessMode(resp.AccessRight),
		MasterDBSize:                resp.BboltSize,
		DerivedDBSize:               resp.BleveSize,
		DocumentDBSize:              resp.DocumentDbSize,
		ActualRepositorySize:        resp.ActualRepositorySize,
		MaxRepositorySize:           resp.MaxRepositorySize,
		NumberOfDatasets:            resp.NumberOfDatasets,
		NumberOfDatasetsInBranch:    resp.NumberOfDatasetsInBranch,
		NumberOfDatasetRevisions:    resp.NumberOfDatasetRevisions,
		NumberOfNamedGraphRevisions: resp.NumberOfNamedGraphRevisions,
		NumberOfCommits:             resp.NumberOfCommits,
		NumberOfRepositoryLogs:      resp.NumberOfRepositoryLogs,
		NumberOfDocuments:           resp.NumberOfDocuments,
		IsRemote:                    true,
		SupportRevisionHistory:      true,
		BleveName:                   bleveInfo.Name,
		BleveVersion:                bleveInfo.Version,
		VersionHash:                 resp.VersionHash,
	}, nil
}

func (r *remoteRepository) Log(ctx context.Context, start, end *int) ([]RepositoryLogEntry, error) {
	if err := r.enter(); err != nil {
		GlobalLogger.Error("", zap.Error(err))
		return nil, err
	}
	defer r.leave()

	req := &bboltproto.GetRepositoryLogRequest{RepoName: r.repoName}

	// only set if start is non-zero (to let nil mean "latest")
	if start != nil && *start != 0 {
		s := int64(*start)
		req.Start = &s
	}
	if end != nil {
		e := int64(*end)
		req.End = &e
	}

	var opts []grpc.CallOption
	if authProvider := sstauth.AuthProviderFromContext(ctx); authProvider != nil {
		if p, ok := authProvider.(sstauth.Provider); ok {
			token, err := p.Oauth2Token()
			if err != nil {
				GlobalLogger.Error("", zap.Error(err))
				return nil, err
			}
			opts = append(opts, grpc.PerRPCCredentials(oauth.NewOauthAccess(token)))
		}
	}

	GlobalLogger.Debug("gRPC dsClient call GetRepositoryLog with range")
	resp, err := r.dsClient.GetRepositoryLog(ctx, req, opts...)
	if err != nil {
		GlobalLogger.Error("", zap.Error(err))
		return nil, err
	}

	sort.SliceStable(resp.Entries, func(i, j int) bool {
		return resp.Entries[i].GetLogKey() > resp.Entries[j].GetLogKey()
	})

	var entries []RepositoryLogEntry
	for _, protoEntry := range resp.Entries {
		fields := make(map[string]string)
		for k, v := range protoEntry.GetFields() {
			fields[k] = v
		}
		entries = append(entries, RepositoryLogEntry{
			LogKey: protoEntry.GetLogKey(),
			Fields: fields,
		})
	}

	return entries, nil
}

func (r *remoteRepository) DocumentSet(ctx context.Context, MIMEType string, source *bufio.Reader) (Hash, error) {
	if err := r.enter(); err != nil {
		return emptyHash, err
	}
	defer r.leave()

	var emptyHash Hash

	var opts []grpc.CallOption
	if authProvider := sstauth.AuthProviderFromContext(ctx); authProvider != nil {
		if p, ok := authProvider.(sstauth.Provider); ok {
			token, err := p.Oauth2Token()
			if err != nil {
				return emptyHash, fmt.Errorf("failed to get oauth token: %w", err)
			}
			opts = append(opts, grpc.PerRPCCredentials(oauth.NewOauthAccess(token)))
		}
	}

	stream, err := r.dsClient.DocumentSet(ctx, opts...)
	if err != nil {
		return emptyHash, fmt.Errorf("failed to initiate upload stream: %w", err)
	}

	// Step 1: send document info
	now := time.Now()
	author := "default@semanticstep.net"
	if user := sstauth.SstUserInfoFromContext(ctx); user != nil && user.Email != "" {
		author = user.Email
	}

	infoMsg := &bboltproto.DocumentSetRequest{
		Event: &bboltproto.DocumentSetRequest_Info{
			Info: &bboltproto.DocumentInfo{
				MimeType:  MIMEType,
				Author:    author,
				Timestamp: timestamppb.New(now),
			},
		},
		RepoName: r.repoName,
	}
	if err := stream.Send(infoMsg); err != nil {
		return emptyHash, fmt.Errorf("failed to send document info: %w", err)
	}

	// Step 2: send content in chunks
	buf := make([]byte, 32*1024) // 32 KB buffer
	for {
		n, readErr := source.Read(buf)
		if n > 0 {
			contentMsg := &bboltproto.DocumentSetRequest{
				Event: &bboltproto.DocumentSetRequest_Content{
					Content: buf[:n],
				},
				RepoName: r.repoName,
			}
			if err := stream.Send(contentMsg); err != nil {
				return emptyHash, fmt.Errorf("failed to send content chunk: %w", err)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return emptyHash, fmt.Errorf("failed to read document content: %w", readErr)
		}
	}

	// Step 3: close and receive hash
	resp, err := stream.CloseAndRecv()
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.ResourceExhausted {
			return emptyHash, fmt.Errorf("%w: %s", ErrQuotaExceeded, s.Message())
		}
		return emptyHash, fmt.Errorf("failed to receive upload response: %w", err)
	}

	if len(resp.GetHash()) != 32 {
		return emptyHash, fmt.Errorf("invalid hash received (length=%d)", len(resp.GetHash()))
	}

	copy(emptyHash[:], resp.GetHash())
	return emptyHash, nil
}

func (r *remoteRepository) Document(ctx context.Context, hash Hash, target io.Writer) (*DocumentInfo, error) {
	if err := r.enter(); err != nil {
		GlobalLogger.Error("", zap.Error(err))
		return nil, err
	}
	defer r.leave()

	req := &bboltproto.DocumentRequest{Hash: hash[:], RepoName: r.repoName}

	var opts []grpc.CallOption
	if authProvider := sstauth.AuthProviderFromContext(ctx); authProvider != nil {
		if p, ok := authProvider.(sstauth.Provider); ok {
			token, err := p.Oauth2Token()
			if err != nil {
				GlobalLogger.Error("", zap.Error(err))
				return nil, err
			}
			opts = append(opts, grpc.PerRPCCredentials(oauth.NewOauthAccess(token)))
		}
	}

	stream, err := r.dsClient.Document(ctx, req, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to initiate document download: %w", err)
	}

	var docInfo *DocumentInfo

	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error receiving document download response: %w", err)
		}

		switch evt := resp.Event.(type) {
		case *bboltproto.DocumentResponse_Info:
			// Server-sent metadata (includes size).
			docInfo = &DocumentInfo{
				Hash:      hash,
				MIMEType:  evt.Info.GetMimeType(),
				Author:    evt.Info.GetAuthor(),
				Timestamp: evt.Info.GetTimestamp().AsTime(),
				Size:      evt.Info.GetSizeBytes(),
			}

		case *bboltproto.DocumentResponse_Chunk:
			if target != nil {
				if _, werr := target.Write(evt.Chunk); werr != nil {
					return nil, fmt.Errorf("failed to write document chunk: %w", werr)
				}
			}
		}
	}

	if docInfo == nil {
		GlobalLogger.Error("", zap.Error(err))
		return nil, errors.New("missing document info in download stream")
	}

	return docInfo, nil
}

func (r *remoteRepository) Documents(ctx context.Context) ([]DocumentInfo, error) {
	if err := r.enter(); err != nil {
		GlobalLogger.Error("", zap.Error(err))
		return nil, err
	}
	defer r.leave()

	req := &bboltproto.DocumentsListRequest{RepoName: r.repoName}

	var opts []grpc.CallOption
	if authProvider := sstauth.AuthProviderFromContext(ctx); authProvider != nil {
		if p, ok := authProvider.(sstauth.Provider); ok {
			token, err := p.Oauth2Token()
			if err != nil {
				GlobalLogger.Error("", zap.Error(err))
				return nil, err
			}
			opts = append(opts, grpc.PerRPCCredentials(oauth.NewOauthAccess(token)))
		}
	}

	resp, err := r.dsClient.Documents(ctx, req, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch documents from remote: %w", err)
	}

	var result []DocumentInfo
	for _, doc := range resp.Documents {
		if len(doc.Hash) != 32 {
			return nil, fmt.Errorf("invalid hash length: %d", len(doc.Hash))
		}

		var h Hash
		copy(h[:], doc.Hash)

		if doc.Timestamp == nil {
			return nil, fmt.Errorf("missing timestamp in document")
		}
		if err := doc.Timestamp.CheckValid(); err != nil {
			return nil, fmt.Errorf("invalid timestamp from server: %w", err)
		}

		result = append(result, DocumentInfo{
			Hash:      h,
			MIMEType:  doc.MimeType,
			Author:    doc.Author,
			Timestamp: doc.Timestamp.AsTime(),
			Size:      doc.GetSizeBytes(),
		})
	}

	return result, nil
}

func (r *remoteRepository) ForDocuments(ctx context.Context, c func(d DocumentInfo) error) error {
	docs, err := r.Documents(ctx)
	if err != nil {
		return err
	}
	for _, d := range docs {
		if err := c(d); err != nil {
			return err
		}
	}
	return nil
}

func (r *remoteRepository) DocumentDelete(ctx context.Context, hash Hash) error {
	if err := r.enter(); err != nil {
		return nil
	}
	defer r.leave()

	req := &bboltproto.DocumentDeleteRequest{
		Hash:     hash[:],
		RepoName: r.repoName,
	}

	var opts []grpc.CallOption
	if authProvider := sstauth.AuthProviderFromContext(ctx); authProvider != nil {
		if p, ok := authProvider.(sstauth.Provider); ok {
			token, err := p.Oauth2Token()
			if err != nil {
				return err
			}
			opts = append(opts, grpc.PerRPCCredentials(oauth.NewOauthAccess(token)))
		}
	}

	_, err := r.dsClient.DocumentDelete(ctx, req, opts...)
	return err
}

func (r *remoteRepository) ExtractSstFile(ctx context.Context, namedGraphRevision Hash, w io.Writer) error {
	if err := r.enter(); err != nil {
		return nil
	}
	defer r.leave()

	req := &bboltproto.DownloadNamedGraphRevisionRequest{
		Hash:     namedGraphRevision[:],
		RepoName: r.repoName,
	}

	var opts []grpc.CallOption
	if authProvider := sstauth.AuthProviderFromContext(ctx); authProvider != nil {
		if p, ok := authProvider.(sstauth.Provider); ok {
			token, err := p.Oauth2Token()
			if err != nil {
				return err
			}
			opts = append(opts, grpc.PerRPCCredentials(oauth.NewOauthAccess(token)))
		}
	}

	stream, err := r.dsClient.DownloadNamedGraphRevision(context.Background(), req, opts...)
	if err != nil {
		return fmt.Errorf("failed to start DownloadNamedGraphRevision: %w", err)
	}

	for {
		resp, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("recv error: %w", err)
		}

		if _, err := w.Write(resp.GetChunk()); err != nil {
			return fmt.Errorf("write error: %w", err)
		}
	}

	return nil
}

// SyncFrom synchronizes data from the source repository to this remote repository.
// For RemoteRepository, syncing from a LocalFullRepository streams bucket data via gRPC.
// Syncing from another RemoteRepository relays SyncTo bucket data into SyncFrom on this repository.
func (r *remoteRepository) SyncFrom(ctx context.Context, from Repository, options ...SyncOption) error {
	if err := r.enter(); err != nil {
		return err
	}
	defer r.leave()

	opts := defaultSyncOptions()
	for _, option := range options {
		option(&opts)
	}

	switch from := from.(type) {
	case *localFullRepository:
		return r.syncFromLocal(ctx, from, opts)
	case *remoteRepository:
		return r.syncFromRemote(ctx, from, opts)
	default:
		return fmt.Errorf("syncing from %T to RemoteRepository is not yet implemented", from)
	}
}

func (r *remoteRepository) syncFromLocal(ctx context.Context, fromLocal *localFullRepository, opts SyncOptions) error {
	datasetIDs := opts.DatasetIDs
	branchName := opts.BranchName

	targetGrpcOpts, err := perRPCCallOpts(ctx)
	if err != nil {
		return fmt.Errorf("failed to get oauth token: %w", err)
	}

	var datasetIDSet map[uuid.UUID]struct{}
	if len(datasetIDs) > 0 {
		datasetIDSet, err = collectAllImportedDatasets(ctx, fromLocal, datasetIDs, branchName)
		if err != nil {
			return fmt.Errorf("failed to collect imported datasets: %w", err)
		}
	}

	GlobalLogger.Debug("gRPC dsClient call SyncFrom")
	stream, err := r.dsClient.SyncFrom(ctx, targetGrpcOpts...)
	if err != nil {
		return fmt.Errorf("failed to initiate SyncFrom stream: %w", err)
	}

	metadata := &bboltproto.SyncFromMetadata{
		RepoName:    r.repoName,
		BranchName:  branchName,
		FromRepoUrl: fromLocal.URL(),
	}
	if len(datasetIDs) > 0 {
		metadata.DatasetIds = make([][]byte, len(datasetIDs))
		for i, dsID := range datasetIDs {
			metadata.DatasetIds[i] = dsID[:]
		}
	}

	if err := stream.Send(&bboltproto.SyncFromRequest{
		Data: &bboltproto.SyncFromRequest_Metadata{
			Metadata: metadata,
		},
	}); err != nil {
		return fmt.Errorf("failed to send metadata: %w", err)
	}

	if err := r.streamBucketData(ctx, fromLocal, stream, datasetIDSet, branchName); err != nil {
		return fmt.Errorf("failed to stream bucket data: %w", err)
	}

	resp, err := stream.CloseAndRecv()
	if err != nil {
		return fmt.Errorf("failed to receive sync response: %w", err)
	}

	GlobalLogger.Debug(
		"SyncFrom completed",
		zap.Int32("namedGraphRevisions", resp.NamedGraphRevisionsSynced),
		zap.Int32("datasetRevisions", resp.DatasetRevisionsSynced),
		zap.Int32("datasets", resp.DatasetsSynced),
		zap.Int32("commits", resp.CommitsSynced),
		zap.Int32("documentInfo", resp.DocumentInfoSynced),
	)

	return nil
}

func (r *remoteRepository) syncFromRemote(ctx context.Context, fromRemote *remoteRepository, opts SyncOptions) error {
	if fromRemote.URL() == r.URL() && fromRemote.repoName == r.repoName {
		return fmt.Errorf("cannot sync from a repository to itself")
	}

	grpcOpts, err := perRPCCallOpts(ctx)
	if err != nil {
		return fmt.Errorf("failed to get oauth token: %w", err)
	}

	datasetIDs := opts.DatasetIDs
	branchName := opts.BranchName

	syncToMetadata := &bboltproto.SyncToMetadata{
		BranchName: branchName,
	}
	if len(datasetIDs) > 0 {
		syncToMetadata.DatasetIds = make([][]byte, len(datasetIDs))
		for i, dsID := range datasetIDs {
			syncToMetadata.DatasetIds[i] = dsID[:]
		}
	}

	GlobalLogger.Debug("gRPC dsClient call SyncTo")
	sourceStream, err := fromRemote.dsClient.SyncTo(ctx, &bboltproto.SyncToRequest{
		Metadata: syncToMetadata,
	}, grpcOpts...)
	if err != nil {
		return fmt.Errorf("failed to initiate SyncTo stream: %w", err)
	}

	GlobalLogger.Debug("gRPC dsClient call SyncFrom")
	targetStream, err := r.dsClient.SyncFrom(ctx, grpcOpts...)
	if err != nil {
		return fmt.Errorf("failed to initiate SyncFrom stream: %w", err)
	}

	syncFromMetadata := &bboltproto.SyncFromMetadata{
		RepoName:    r.repoName,
		BranchName:  branchName,
		FromRepoUrl: fromRemote.URL(),
	}
	if len(datasetIDs) > 0 {
		syncFromMetadata.DatasetIds = make([][]byte, len(datasetIDs))
		for i, dsID := range datasetIDs {
			syncFromMetadata.DatasetIds[i] = dsID[:]
		}
	}

	if err := targetStream.Send(&bboltproto.SyncFromRequest{
		Data: &bboltproto.SyncFromRequest_Metadata{
			Metadata: syncFromMetadata,
		},
	}); err != nil {
		return fmt.Errorf("failed to send metadata: %w", err)
	}

	documentHashesToSync := make(map[string]bool)

	for {
		resp, err := sourceStream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to receive sync data from source: %w", err)
		}

		switch data := resp.Data.(type) {
		case *bboltproto.SyncToResponse_BucketData:
			bd := data.BucketData
			if bd != nil && bd.BucketName == "document_info" && len(bd.Key) > 0 {
				documentHashesToSync[string(bd.Key)] = true
			}
			if err := targetStream.Send(&bboltproto.SyncFromRequest{
				Data: &bboltproto.SyncFromRequest_BucketData{
					BucketData: syncToBucketDataAsSyncFrom(bd),
				},
			}); err != nil {
				return fmt.Errorf("failed to send bucket data to target: %w", err)
			}
		case *bboltproto.SyncToResponse_Complete:
			GlobalLogger.Debug(
				"SyncTo completed",
				zap.Int32("namedGraphRevisions", data.Complete.NamedGraphRevisionsSynced),
				zap.Int32("datasetRevisions", data.Complete.DatasetRevisionsSynced),
				zap.Int32("datasets", data.Complete.DatasetsSynced),
				zap.Int32("commits", data.Complete.CommitsSynced),
				zap.Int32("documentInfo", data.Complete.DocumentInfoSynced),
			)
		}
	}

	resp, err := targetStream.CloseAndRecv()
	if err != nil {
		return fmt.Errorf("failed to receive sync response: %w", err)
	}

	GlobalLogger.Debug(
		"SyncFrom completed",
		zap.Int32("namedGraphRevisions", resp.NamedGraphRevisionsSynced),
		zap.Int32("datasetRevisions", resp.DatasetRevisionsSynced),
		zap.Int32("datasets", resp.DatasetsSynced),
		zap.Int32("commits", resp.CommitsSynced),
		zap.Int32("documentInfo", resp.DocumentInfoSynced),
	)

	if err := syncDocumentFilesBetweenRemotes(ctx, fromRemote, r, documentHashesToSync); err != nil {
		GlobalLogger.Warn("failed to sync some document files between remotes", zap.Error(err))
	}

	return nil
}

func syncToBucketDataAsSyncFrom(bd *bboltproto.SyncToBucketData) *bboltproto.SyncFromBucketData {
	if bd == nil {
		return nil
	}
	return &bboltproto.SyncFromBucketData{
		BucketName: bd.BucketName,
		Key:        bd.Key,
		Value:      bd.Value,
		IsBucket:   bd.IsBucket,
		SubKey:     bd.SubKey,
		SubValue:   bd.SubValue,
	}
}

func syncDocumentFilesBetweenRemotes(
	ctx context.Context,
	fromRemote, toRemote *remoteRepository,
	documentHashes map[string]bool,
) error {
	for hashStr := range documentHashes {
		hash, err := StringToHash(hashStr)
		if err != nil {
			GlobalLogger.Warn("invalid document hash during remote sync", zap.String("hash", hashStr), zap.Error(err))
			continue
		}

		var buf bytes.Buffer
		docInfo, err := fromRemote.Document(ctx, hash, &buf)
		if err != nil {
			GlobalLogger.Warn("failed to read document from source remote repository during sync",
				zap.Error(err),
				zap.String("hash", hash.String()))
			continue
		}

		if _, err := toRemote.DocumentSet(ctx, docInfo.MIMEType, bufio.NewReader(&buf)); err != nil {
			GlobalLogger.Warn("failed to store document to target remote repository during sync",
				zap.Error(err),
				zap.String("hash", hash.String()))
			continue
		}

		GlobalLogger.Debug("synced document file between remotes",
			zap.String("hash", hash.String()),
			zap.String("mime_type", docInfo.MIMEType))
	}

	return nil
}

// streamBucketData streams all bucket data from local repository to remote.
// If datasetIDSet is not nil, only streams data related to the specified datasets.
// If branchName is not empty, only streams data from the specified branch.
func (r *remoteRepository) streamBucketData(
	ctx context.Context,
	fromLocal *localFullRepository,
	stream bboltproto.DatasetService_SyncFromClient,
	datasetIDSet map[uuid.UUID]struct{},
	branchName string,
) error {
	return fromLocal.db.View(func(tx *bbolt.Tx) error {
		// Get buckets needed for filtering
		var dsrBucket, datasetsBucket, commitsBucket *bbolt.Bucket
		if datasetIDSet != nil || !isAllBranches(branchName) {
			dsrBucket = tx.Bucket(keyDatasetRevisions)
			datasetsBucket = tx.Bucket(keyDatasets)
			commitsBucket = tx.Bucket(keyCommits)
		}

		// 1. Stream NamedGraphRevisions
		if err := r.streamNamedGraphRevisions(tx, stream, datasetIDSet, dsrBucket, datasetsBucket, commitsBucket, branchName); err != nil {
			return fmt.Errorf("failed to stream NamedGraphRevisions: %w", err)
		}

		// 2. Stream DatasetRevisions
		if err := r.streamDatasetRevisions(tx, stream, datasetIDSet, datasetsBucket, commitsBucket, branchName); err != nil {
			return fmt.Errorf("failed to stream DatasetRevisions: %w", err)
		}

		// 3. Stream Datasets
		if err := r.streamDatasets(tx, stream, datasetIDSet, branchName); err != nil {
			return fmt.Errorf("failed to stream Datasets: %w", err)
		}

		// 4. Stream Commits
		if err := r.streamCommits(tx, stream, datasetIDSet, branchName); err != nil {
			return fmt.Errorf("failed to stream Commits: %w", err)
		}

		// 5. Stream document_info
		if err := r.streamDocumentInfo(tx, stream, datasetIDSet); err != nil {
			return fmt.Errorf("failed to stream document_info: %w", err)
		}

		return nil
	})
}

func (r *remoteRepository) streamNamedGraphRevisions(tx *bbolt.Tx, stream bboltproto.DatasetService_SyncFromClient, datasetIDSet map[uuid.UUID]struct{}, dsrBucket *bbolt.Bucket, datasetsBucket *bbolt.Bucket, commitsBucket *bbolt.Bucket, branchName string) error {
	bucket := tx.Bucket(keyNamedGraphRevisions)
	if bucket == nil {
		return nil
	}

	return bucket.ForEach(func(k, v []byte) error {
		// Filter by datasetIDSet and/or branchName if provided
		if datasetIDSet != nil || !isAllBranches(branchName) {
			ngRevisionHash := BytesToHash(k)
			if !isNamedGraphRevisionForDatasets(ngRevisionHash, datasetIDSet, branchName, dsrBucket, datasetsBucket, commitsBucket) {
				return nil // Not for specified datasets/branch, skip
			}
		}

		return stream.Send(&bboltproto.SyncFromRequest{
			Data: &bboltproto.SyncFromRequest_BucketData{
				BucketData: &bboltproto.SyncFromBucketData{
					BucketName: "ngr",
					Key:        k,
					Value:      v,
					IsBucket:   false,
				},
			},
		})
	})
}

func (r *remoteRepository) streamDatasetRevisions(tx *bbolt.Tx, stream bboltproto.DatasetService_SyncFromClient, datasetIDSet map[uuid.UUID]struct{}, datasetsBucket *bbolt.Bucket, commitsBucket *bbolt.Bucket, branchName string) error {
	bucket := tx.Bucket(keyDatasetRevisions)
	if bucket == nil {
		return nil
	}

	return bucket.ForEach(func(k, v []byte) error {
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

		// Send each sub-bucket entry
		return subBucket.ForEach(func(sk, sv []byte) error {
			return stream.Send(&bboltproto.SyncFromRequest{
				Data: &bboltproto.SyncFromRequest_BucketData{
					BucketData: &bboltproto.SyncFromBucketData{
						BucketName: "dsr",
						Key:        k,
						Value:      nil,
						IsBucket:   true,
						SubKey:     sk,
						SubValue:   sv,
					},
				},
			})
		})
	})
}

func (r *remoteRepository) streamDatasets(tx *bbolt.Tx, stream bboltproto.DatasetService_SyncFromClient, datasetIDSet map[uuid.UUID]struct{}, branchName string) error {
	bucket := tx.Bucket(keyDatasets)
	if bucket == nil {
		return nil
	}

	return bucket.ForEach(func(k, v []byte) error {
		dsBucket := bucket.Bucket(k)
		if dsBucket == nil {
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
			branchValue := dsBucket.Get(branchKey)
			if branchValue == nil {
				// Branch not found for this dataset, skip
				return nil
			}
			// Send only the specified branch entry
			return stream.Send(&bboltproto.SyncFromRequest{
				Data: &bboltproto.SyncFromRequest_BucketData{
					BucketData: &bboltproto.SyncFromBucketData{
						BucketName: "ds",
						Key:        k,
						Value:      nil,
						IsBucket:   true,
						SubKey:     branchKey,
						SubValue:   branchValue,
					},
				},
			})
		}

		// Send each dataset entry (branch/commit references)
		return dsBucket.ForEach(func(sk, sv []byte) error {
			return stream.Send(&bboltproto.SyncFromRequest{
				Data: &bboltproto.SyncFromRequest_BucketData{
					BucketData: &bboltproto.SyncFromBucketData{
						BucketName: "ds",
						Key:        k,
						Value:      nil,
						IsBucket:   true,
						SubKey:     sk,
						SubValue:   sv,
					},
				},
			})
		})
	})
}

func (r *remoteRepository) streamCommits(tx *bbolt.Tx, stream bboltproto.DatasetService_SyncFromClient, datasetIDSet map[uuid.UUID]struct{}, branchName string) error {
	bucket := tx.Bucket(keyCommits)
	if bucket == nil {
		return nil
	}

	// Get datasetsBucket for branch filtering
	var datasetsBucket *bbolt.Bucket
	if !isAllBranches(branchName) {
		datasetsBucket = tx.Bucket(keyDatasets)
	}

	return bucket.ForEach(func(k, v []byte) error {
		commitBucket := bucket.Bucket(k)
		if commitBucket == nil {
			return nil
		}

		commitHash := BytesToHash(k)

		// Filter by branch if specified
		if !isAllBranches(branchName) && datasetsBucket != nil {
			if !isCommitInBranch(commitHash, branchName, datasetIDSet, datasetsBucket, bucket) {
				return nil // Not in specified branch, skip
			}
		}

		// Filter by datasetIDSet if provided
		if datasetIDSet != nil {
			if !isCommitForDatasets(commitHash, datasetIDSet, commitBucket) {
				return nil // Not for specified datasets, skip
			}
		}

		// Send each commit entry
		return commitBucket.ForEach(func(sk, sv []byte) error {
			return stream.Send(&bboltproto.SyncFromRequest{
				Data: &bboltproto.SyncFromRequest_BucketData{
					BucketData: &bboltproto.SyncFromBucketData{
						BucketName: "c",
						Key:        k,
						Value:      nil,
						IsBucket:   true,
						SubKey:     sk,
						SubValue:   sv,
					},
				},
			})
		})
	})
}

func (r *remoteRepository) streamDocumentInfo(tx *bbolt.Tx, stream bboltproto.DatasetService_SyncFromClient, datasetIDSet map[uuid.UUID]struct{}) error {
	// If datasetIDSet is provided, skip syncing documents
	if datasetIDSet != nil {
		return nil
	}

	bucket := tx.Bucket([]byte(documentInfoBucket))
	if bucket == nil {
		return nil
	}

	return bucket.ForEach(func(k, v []byte) error {
		docBucket := bucket.Bucket(k)
		if docBucket == nil {
			return nil
		}

		// Send each document info entry
		return docBucket.ForEach(func(sk, sv []byte) error {
			return stream.Send(&bboltproto.SyncFromRequest{
				Data: &bboltproto.SyncFromRequest_BucketData{
					BucketData: &bboltproto.SyncFromBucketData{
						BucketName: "document_info",
						Key:        k,
						Value:      nil,
						IsBucket:   true,
						SubKey:     sk,
						SubValue:   sv,
					},
				},
			})
		})
	})
}

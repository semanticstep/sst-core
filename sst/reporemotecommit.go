// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"log"
	"sort"

	"github.com/semanticstep/sst-core/bboltproto"
	"github.com/semanticstep/sst-core/sstauth"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type commitServiceServer struct {
	r  Repository
	sr SuperRepository
	bboltproto.UnimplementedCommitServiceServer
}

// To be Removed/Used
func (s *commitServiceServer) ListCommits(stream bboltproto.CommitService_ListCommitsServer) error {
	GlobalLogger.Debug("gRPC commitServiceServer received ListCommits request")
	panic("not used")
}

func (s *commitServiceServer) GetCommit(stream bboltproto.CommitService_GetCommitServer) error {
	log.Panic("GetCommit is not used, use GetCommitDetailsBatch instead")
	return nil
}

type idsAndHashes struct {
	sortedGraphIDs     []uuid.UUID
	sortedGraphHashes  []Hash
	sortedGraphContent [][]byte
}

func (s *idsAndHashes) Len() int { return len(s.sortedGraphIDs) }

func (s *idsAndHashes) Less(i int, j int) bool {
	return bytes.Compare(s.sortedGraphIDs[i][:], s.sortedGraphIDs[j][:]) < 0
}

func (s *idsAndHashes) Swap(i int, j int) {
	s.sortedGraphIDs[i], s.sortedGraphIDs[j] = s.sortedGraphIDs[j], s.sortedGraphIDs[i]
	s.sortedGraphHashes[i], s.sortedGraphHashes[j] = s.sortedGraphHashes[j], s.sortedGraphHashes[i]
	s.sortedGraphContent[i], s.sortedGraphContent[j] = s.sortedGraphContent[j], s.sortedGraphContent[i]
}

func (s *commitServiceServer) CreateCommit(ctx context.Context, request *bboltproto.CreateCommitRequest) (*bboltproto.CreateCommitResponse, error) {
	GlobalLogger.Debug("CreateCommit request Received")

	repo, err := commitServiceServerToRepository(s, ctx, request.RepoName)
	if err != nil {
		return nil, err
	}

	// extract sstUserInfo from context and set it to localFullRepository.authInfo
	userInfo := sstauth.SstUserInfoFromContext(ctx)
	if userInfo == nil {
		GlobalLogger.Error("CreateCommit: no UserInfo in context, set to default user")
	} else {
		GlobalLogger.Debug("CreateCommit: user:", zap.String("email", userInfo.Email))
		repo.(*localFullRepository).authInfo = userInfo
	}

	var branch string
	switch refName := request.RefName.RefName.(type) {
	case *bboltproto.RefName_Branch:
		branch = refName.Branch
	default:
		GlobalLogger.Error("CreateCommit: refName is not a branch")
	}
	ngs := idsAndHashes{
		sortedGraphIDs:     make([]uuid.UUID, 0, len(request.NamedGraphs)),
		sortedGraphHashes:  make([]Hash, 0, len(request.NamedGraphs)),
		sortedGraphContent: make([][]byte, 0, len(request.NamedGraphs)),
	}
	for _, ngr := range request.NamedGraphs {
		graph, err := sstReadGraphImports(bufio.NewReader(bytes.NewBuffer(ngr)))
		if err != nil {
			return nil, err
		}
		ngs.sortedGraphIDs = append(ngs.sortedGraphIDs, graph.ID())
		ngs.sortedGraphContent = append(ngs.sortedGraphContent, ngr)
		ngRev := sha256.Sum256(ngr)
		ngs.sortedGraphHashes = append(ngs.sortedGraphHashes, ngRev)
	}
	sort.Sort(&ngs)
	newStorageStage := repo.OpenStage(DefaultTriplexMode).(*stage)
	for i, ngContent := range ngs.sortedGraphContent {
		currentNGID := ngs.sortedGraphIDs[i]
		dgFile := byteFileReader{keyedFile: keyedFile{currentNGID}, byteReader: bytes.NewReader(ngContent)}
		defer dgFile.Close()

		ng, err := SstRead(bufio.NewReader(dgFile), DefaultTriplexMode)
		if err != nil {
			panic(err)
		}

		// ng.ForIRINodes(func(d IBNode) error {
		// 	if d.TypeOf() != nil {
		// 		fmt.Println(d.IRI(), d.TypeOf().IRI())
		// 	} else {
		// 		fmt.Println(d.IRI(), d.TypeOf())
		// 	}
		// 	return nil
		// })
		_, err = newStorageStage.MoveAndMerge(context.TODO(), ng.Stage())
		if err != nil {
			panic(err)
		}
	}
	// fmt.Println("------------------------------------------------------------------------")

	// correct modified flag
	for _, ng := range newStorageStage.localGraphs {
		ng.flags.modified = false
	}

	for _, modifiedID := range request.ModifiedNGs {
		if _, ok := newStorageStage.localGraphs[uuid.UUID(modifiedID)]; ok {
			newStorageStage.localGraphs[uuid.UUID(modifiedID)].flags.modified = true
		}
	}

	// correct NG and DS Signatures
	for _, NamedGraphSignature := range request.CheckoutNamedGraphSignatures {
		if ng, ok := newStorageStage.localGraphs[uuid.UUID(NamedGraphSignature.NgUuid)]; ok {
			ng.checkedOutNGRevisions = []Hash{Hash(NamedGraphSignature.NgHash)}
		}
	}
	for _, DatasetSignature := range request.CheckoutDatasetSignatures {
		if ng, ok := newStorageStage.localGraphs[uuid.UUID(DatasetSignature.DSUuid)]; ok {
			ng.checkedOutDSRevisions = []Hash{Hash(DatasetSignature.DSHash)}
		}
	}
	for _, checkoutCommitHash := range request.ParentCommitOverrides {
		if ng, ok := newStorageStage.localGraphs[uuid.UUID(checkoutCommitHash.DatasetUuid)]; ok {
			parents := make([]Hash, 0, len(checkoutCommitHash.CommitId)/len(Hash{}))
			for i := 0; i < len(checkoutCommitHash.CommitId); i += len(Hash{}) {
				parents = append(parents, BytesToHash(checkoutCommitHash.CommitId[i:i+len(Hash{})]))
			}
			ng.checkedOutCommits = parents
		}
	}

	// for _, val := range newStorageStage.localGraphs {
	// 	val.ForIRINodes(func(d IBNode) error {
	// 		if d.TypeOf() != nil {
	// 			fmt.Println(d.IRI(), d.TypeOf().IRI())
	// 		} else {
	// 			fmt.Println(d.IRI(), d.TypeOf())
	// 		}
	// 		return nil
	// 	})
	// }

	newCommitID, modifiedDSIDs, err := newStorageStage.Commit(context.TODO(), request.Message, branch)
	if err != nil {
		if errors.Is(err, ErrQuotaExceeded) {
			return nil, status.Errorf(codes.ResourceExhausted, "commit failed: %v", err)
		}
		return &bboltproto.CreateCommitResponse{CommitId: newCommitID[:]}, err
	}

	// Ensure asynchronous Bleve index updates are visible before returning to
	// the remote client.
	FlushBleveIndex(repo)

	// after commit, return generated NGHashes to GRPC client to update the NG checkedOutNGHash in client side
	checkoutNamedGraphSignatures := make([]*bboltproto.NamedGraphSignature, 0, len(request.ModifiedNGs))
	for _, modifiedID := range request.ModifiedNGs {
		if ng, ok := newStorageStage.localGraphs[uuid.UUID(modifiedID)]; ok {
			lastNGRev := ng.lastCheckedOutNGRevision()
			checkoutNamedGraphSignatures = append(checkoutNamedGraphSignatures, &bboltproto.NamedGraphSignature{
				NgUuid: ng.id[:],
				NgHash: lastNGRev[:],
			})
		}
	}

	checkoutDatasetSignatures := make([]*bboltproto.DatasetSignature, 0, len(modifiedDSIDs))
	for _, modifiedDsID := range modifiedDSIDs {
		if ng, ok := newStorageStage.localGraphs[uuid.UUID(modifiedDsID)]; ok {
			lastDSRev := ng.lastCheckedOutDSRevision()
			checkoutDatasetSignatures = append(checkoutDatasetSignatures, &bboltproto.DatasetSignature{
				DSUuid: ng.id[:],
				DSHash: lastDSRev[:],
			})
		}
	}

	modifiedDatasets := make([][]byte, 0)
	for _, ID := range modifiedDSIDs {
		modifiedDatasets = append(modifiedDatasets, ID[:])
	}

	return &bboltproto.CreateCommitResponse{
		CommitId:                     newCommitID[:],
		CheckoutNamedGraphSignatures: checkoutNamedGraphSignatures,
		CheckoutDatasetSignatures:    checkoutDatasetSignatures,
		ModifiedDatasets:             modifiedDatasets,
	}, nil
}

func (s *commitServiceServer) CompareCommits(ctx context.Context, request *bboltproto.CompareCommitsRequest) (*bboltproto.CompareCommitsResponse, error) {
	GlobalLogger.Debug("gRPC commitServiceServer received CompareCommits request")

	repo, err := commitServiceServerToRepository(s, ctx, request.RepoName)
	if err != nil {
		return nil, err
	}

	commitPairs := make([]datasetCommitPair, 0, len(request.DatasetCommitPair))
	for _, p := range request.DatasetCommitPair {
		dsIDs := make([]uuid.UUID, 0, len(p.DatasetId))
		for _, dsIDBytes := range p.DatasetId {
			dsIDs = append(dsIDs, uuid.UUID(*(*[len(uuid.UUID{})]byte)(dsIDBytes)))
		}
		commitPairs = append(commitPairs, datasetCommitPair{
			DsIDs: dsIDs,
			Pair: [2]Hash{
				Hash(*(*[len(Hash{})]byte)(p.Commit1)),
				Hash(*(*[len(Hash{})]byte)(p.Commit2)),
			},
		})
	}
	commitRel, err := compareCommits(repo, commitPairs)
	if err != nil {
		return nil, err
	}
	relationshipAndCount := make([]byte, 0, len(commitRel)<<1)
	for _, rr := range commitRel {
		relationshipAndCount = append(relationshipAndCount, []byte{byte(rr.Relationship), rr.Count}...)
	}
	return &bboltproto.CompareCommitsResponse{RelationshipAndCount: relationshipAndCount}, nil
}

func (s *commitServiceServer) GetCommitDetailsBatch(
	ctx context.Context,
	req *bboltproto.GetCommitDetailsBatchRequest,
) (*bboltproto.GetCommitDetailsBatchResponse, error) {
	var hashes []Hash
	for _, idBytes := range req.CommitIds {
		if len(idBytes) != 32 {
			continue // skip invalid IDs
		}
		var h Hash
		copy(h[:], idBytes)
		hashes = append(hashes, h)
	}

	repo, err := commitServiceServerToRepository(s, ctx, req.RepoName)
	if err != nil {
		return nil, err
	}

	detailsList, err := repo.CommitDetails(ctx, hashes)
	if err != nil {
		return nil, err
	}

	var protoList []*bboltproto.CommitDetail
	for _, d := range detailsList {
		protoList = append(protoList, commitDetailsToCommitDetailProto(d))
	}

	return &bboltproto.GetCommitDetailsBatchResponse{
		Details: protoList,
	}, nil
}

func commitDetailsToCommitDetailProto(details *CommitDetails) *bboltproto.CommitDetail {
	return &bboltproto.CommitDetail{
		CommitId:            details.Commit[:],
		Author:              details.Author,
		Message:             details.Message,
		Timestamp:           details.AuthorDate.Unix(),
		ParentCommits:       convertParentCommits(details.ParentCommits),
		DatasetRevisions:    convertIRIHashMap(details.DatasetRevisions),
		NamedGraphRevisions: convertIRIHashMap(details.NamedGraphRevisions),
	}
}

func convertParentCommits(parents map[IRI][]Hash) map[string]*bboltproto.ParentHashes {
	result := make(map[string]*bboltproto.ParentHashes)
	for iri, hashes := range parents {
		var hashBytes [][]byte
		for _, h := range hashes {
			hashCopy := make([]byte, 32)
			copy(hashCopy, h[:])
			hashBytes = append(hashBytes, hashCopy)
		}
		result[string(iri)] = &bboltproto.ParentHashes{
			Hashes: hashBytes,
		}
	}
	return result
}

func convertIRIHashMap(m map[IRI]Hash) map[string][]byte {
	out := make(map[string][]byte)
	for k, v := range m {
		b := make([]byte, 32)
		copy(b, v[:])
		out[string(k)] = b
	}
	return out
}

func commitServiceServerToRepository(s *commitServiceServer, ctx context.Context, repoName string) (Repository, error) {
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

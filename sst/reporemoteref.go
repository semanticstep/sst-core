// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst

import (
	"context"
	"errors"

	"github.com/semanticstep/sst-core/bboltproto"
	"github.com/google/uuid"
)

var (
	errBreak = errors.New("break")
)

type refServiceServer struct {
	R  Repository
	sr SuperRepository
	bboltproto.UnimplementedRefServiceServer
}

func (s *refServiceServer) ListRefs(
	ctx context.Context, request *bboltproto.ListRefsRequest,
) (_ *bboltproto.ListRefsResponse, err error) {
	var response bboltproto.ListRefsResponse
	return &response, nil
}

func (s *refServiceServer) GetRef(stream bboltproto.RefService_GetRefServer) error {
	request, err := stream.Recv()
	if err != nil {
		return err
	}
	fmFilter := request.FieldMask_Filter()
	unmaskCommitID, unmaskRef := fmFilter.MaskedOut_CommitId(), fmFilter.MaskedOut_Ref()
	dsID, err := uuid.FromBytes(request.RefName.GetDatasetUuid())
	if err != nil {
		return err
	}

	repo, err := refServiceServerToRepository(s, stream.Context(), request.RepoName)
	if err != nil {
		return err
	}

	ds, err := repo.Dataset(context.TODO(), IRI(dsID.URN()))
	if err != nil {
		return err
	}
	var response bboltproto.Ref
	var brName string
	var leaf []byte
	var cd *CommitDetails
	var nilCommitID bool
	switch refName := request.RefName.RefName.(type) {
	case *bboltproto.RefName_Branch:
		brName = refName.Branch
		cd, err = ds.CommitDetailsByBranch(context.TODO(), brName)
		if err != nil {
			return err
		}
	case *bboltproto.RefName_Leaf:
		leaf = refName.Leaf
		cd, err = ds.CommitDetailsByHash(context.TODO(), Hash(leaf))
		if err != nil {
			return err
		}
	}
	if unmaskCommitID {
		if nilCommitID {
			emptyHash := HashNil()
			response.CommitId = emptyHash[:]
		} else {
			response.CommitId = cd.Commit[:]
		}
	}
	if unmaskRef {
		var refName bboltproto.RefName
		unmaskBranch, unmaskLeaf := fmFilter.MaskedOut_Ref_Branch(), fmFilter.MaskedOut_Ref_Leaf()
		if unmaskBranch && brName != "" {
			refName.RefName = &bboltproto.RefName_Branch{Branch: brName}
		}
		if unmaskLeaf && leaf != nil {
			refName.RefName = &bboltproto.RefName_Leaf{Leaf: leaf}
		}
		unmaskUUID, unmaskIri := fmFilter.MaskedOut_Ref_DatasetUuid(), fmFilter.MaskedOut_Ref_DatasetIri()
		if unmaskUUID {
			refName.DatasetId = &bboltproto.RefName_DatasetUuid{DatasetUuid: dsID[:]}
		} else if unmaskIri {
			refName.DatasetId = &bboltproto.RefName_DatasetIri{DatasetIri: ds.IRI().String()}
		}
		response.Ref = &refName
	}
	return stream.SendAndClose(&response)
}

func refServiceServerToRepository(s *refServiceServer, ctx context.Context, repoName string) (Repository, error) {
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
		repo = s.R
	}

	if repo == nil {
		return nil, ErrRepoNotFound
	}

	return repo, nil
}

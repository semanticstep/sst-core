// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst

import (
	"context"
	"sort"
)

func checkoutCommitFromCommitDetails(ctx context.Context, repo Repository, commitID Hash, mode TriplexMode, cd *CommitDetails) (Stage, error) {
	if len(cd.DatasetRevisions) == 0 {
		st := repo.OpenStage(mode)
		if st == nil {
			return nil, wrapError(ErrRepoClosed)
		}
		return st, nil
	}

	dsIRIs := make([]IRI, 0, len(cd.DatasetRevisions))
	for iri := range cd.DatasetRevisions {
		dsIRIs = append(dsIRIs, iri)
	}
	sort.Slice(dsIRIs, func(i, j int) bool { return dsIRIs[i] < dsIRIs[j] })

	var result Stage
	for _, dsIRI := range dsIRIs {
		ds, err := repo.Dataset(ctx, dsIRI)
		if err != nil {
			return nil, wrapError(err)
		}
		st, err := ds.CheckoutCommit(ctx, commitID, mode)
		if err != nil {
			return nil, wrapError(err)
		}
		if result == nil {
			result = st
			continue
		}
		if _, err = result.MoveAndMerge(ctx, st); err != nil {
			return nil, wrapError(err)
		}
	}
	return result, nil
}

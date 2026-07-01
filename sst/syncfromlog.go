// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/semanticstep/sst-core/sstauth"
	"go.etcd.io/bbolt"
)

type syncFromAffectedEntry struct {
	datasetIRI string
	branch     string
	dsRevision string
}

func snapshotExistingDSRs(db *bbolt.DB) (map[Hash]struct{}, error) {
	snapshot := make(map[Hash]struct{})
	err := db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(keyDatasetRevisions)
		if bucket == nil {
			return nil
		}
		return bucket.ForEach(func(k, _ []byte) error {
			if bucket.Bucket(k) != nil {
				snapshot[BytesToHash(k)] = struct{}{}
			}
			return nil
		})
	})
	return snapshot, err
}

func dsRevisionFromCommit(commitsBucket *bbolt.Bucket, dsID uuid.UUID, commitHash Hash) (Hash, bool) {
	if commitsBucket == nil {
		return emptyHash, false
	}
	commitBucket := commitsBucket.Bucket(commitHash[:])
	if commitBucket == nil {
		return emptyHash, false
	}
	dsKey := iDToPrefixedKey(dsID, commitDsPrefix)
	dsHash := commitBucket.Get(dsKey)
	if dsHash == nil || len(dsHash) < len(hashT{}) {
		return emptyHash, false
	}
	return BytesToHash(dsHash[:len(hashT{})]), true
}

func collectSyncFromAffectedEntries(
	tx *bbolt.Tx,
	existingDSRs map[Hash]struct{},
	branchName string,
) ([]syncFromAffectedEntry, error) {
	dsrBucket := tx.Bucket(keyDatasetRevisions)
	datasetsBucket := tx.Bucket(keyDatasets)
	commitsBucket := tx.Bucket(keyCommits)
	if dsrBucket == nil || datasetsBucket == nil {
		return nil, nil
	}

	newDSRs := make(map[Hash]struct{})
	if err := dsrBucket.ForEach(func(k, _ []byte) error {
		if dsrBucket.Bucket(k) == nil {
			return nil
		}
		h := BytesToHash(k)
		if _, existed := existingDSRs[h]; !existed {
			newDSRs[h] = struct{}{}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	if len(newDSRs) == 0 {
		return nil, nil
	}

	datasetsToScan := make(map[uuid.UUID]struct{})
	if err := datasetsBucket.ForEach(func(k, _ []byte) error {
		if datasetsBucket.Bucket(k) != nil {
			datasetsToScan[uuid.UUID(k)] = struct{}{}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	seen := make(map[string]struct{})
	var entries []syncFromAffectedEntry

	for dsID := range datasetsToScan {
		dsBucket := datasetsBucket.Bucket(dsID[:])
		if dsBucket == nil {
			continue
		}
		datasetIRI := getDatasetIRI(dsBucket, dsID)
		if datasetIRI == "" {
			datasetIRI = dsID.URN()
		}

		if err := dsBucket.ForEach(func(k, v []byte) error {
			if len(k) == 0 {
				return nil
			}

			var branch string
			switch refPrefix(k[0]) {
			case refBranchPrefix:
				branch = string(k[1:])
				if !isAllBranches(branchName) && branch != branchName {
					return nil
				}
			case refLeafPrefix:
				if !isAllBranches(branchName) {
					return nil
				}
				branch = "leaf"
			default:
				if k[0] == dsIRIPrefix {
					return nil
				}
				return nil
			}

			if len(v) != len(Hash{}) {
				return nil
			}
			commitHash := BytesToHash(v)
			dsRevision, ok := dsRevisionFromCommit(commitsBucket, dsID, commitHash)
			if !ok {
				return nil
			}
			if _, isNew := newDSRs[dsRevision]; !isNew {
				return nil
			}

			key := fmt.Sprintf("%s\x00%s\x00%s", datasetIRI, branch, dsRevision.String())
			if _, dup := seen[key]; dup {
				return nil
			}
			seen[key] = struct{}{}
			entries = append(entries, syncFromAffectedEntry{
				datasetIRI: datasetIRI,
				branch:     branch,
				dsRevision: dsRevision.String(),
			})
			return nil
		}); err != nil {
			return nil, err
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].datasetIRI != entries[j].datasetIRI {
			return entries[i].datasetIRI < entries[j].datasetIRI
		}
		if entries[i].branch != entries[j].branch {
			return entries[i].branch < entries[j].branch
		}
		return entries[i].dsRevision < entries[j].dsRevision
	})

	return entries, nil
}

func writeSyncFromLogEntry(ctx context.Context, db *bbolt.DB, fromRepoURL string, affected []syncFromAffectedEntry) error {
	if len(affected) == 0 {
		return nil
	}

	author := "default@semanticstep.net"
	if u := sstauth.SstUserInfoFromContext(ctx); u != nil && u.Email != "" {
		author = u.Email
	}

	fields := map[string]string{
		"type":           "sync_from",
		"message":        "sync from repository",
		"author":         author,
		"timestamp":      time.Now().UTC().Format(time.RFC3339),
		"from_repo_url":  fromRepoURL,
		"affected_count": fmt.Sprintf("%d", len(affected)),
	}
	for i, entry := range affected {
		idx := fmt.Sprintf("%d", i)
		fields["dataset_"+idx] = entry.datasetIRI
		fields["branch_"+idx] = entry.branch
		fields["ds_revision_"+idx] = entry.dsRevision
	}

	return db.Update(func(tx *bbolt.Tx) error {
		repoLogBucket, err := tx.CreateBucketIfNotExists([]byte("log"))
		if err != nil {
			return err
		}
		return writeRepositoryLogEntry(repoLogBucket, fields)
	})
}

func writeSyncFromLogAfterSync(
	ctx context.Context,
	r *localFullRepository,
	fromRepoURL string,
	existingDSRs map[Hash]struct{},
	branchName string,
) error {
	var affected []syncFromAffectedEntry
	err := r.db.View(func(tx *bbolt.Tx) error {
		var collectErr error
		affected, collectErr = collectSyncFromAffectedEntries(tx, existingDSRs, branchName)
		return collectErr
	})
	if err != nil {
		return fmt.Errorf("failed to collect sync_from log entries: %w", err)
	}
	return writeSyncFromLogEntry(ctx, r.db, fromRepoURL, affected)
}

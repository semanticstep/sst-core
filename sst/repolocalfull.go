// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/google/uuid"
	fs "github.com/relab/wrfs"
	"github.com/semanticstep/sst-core/bboltproto"
	"github.com/semanticstep/sst-core/sstauth"
	"go.etcd.io/bbolt"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/oauth"
)

type (
	hashT [sha256.Size]byte
	// upgradeRef interface{ UpgradeRef() CommitRef }.
)

var (
	errCommitAuthorNotAvailable   = errors.New("commit author not available")
	errDatasetRevisionMissing     = errors.New("dataset revision is missing")
	errBucketNotExist             = errors.New("bucket not exist")
	errUnknownValue               = errors.New("unknown value")
	errCommitHasNoDatasetRevision = errors.New("commit has no dataset revision")
	ErrSameRevisions              = errors.New("expected different revisions provided")
	// This means the bbolt name and index name are fixed for every repository.
	indexBleveDir = "index.bleve"
	bboltName     = "bbolt.db"
	bleveInfoName = "bleveInfo.json"
)

// createLeafCommitRef creates a leaf commit reference in the dataset bucket.
// Key: refLeafPrefix (0x02) + commitHash, Value: commitHash.
func createLeafCommitRef(dsBucket *bbolt.Bucket, commitHash []byte) error {
	leafKey := bytesToRefKey(commitHash, refLeafPrefix)
	return dsBucket.Put(leafKey, commitHash)
}

// deleteLeafCommitRef deletes a leaf commit reference from the dataset bucket.
func deleteLeafCommitRef(dsBucket *bbolt.Bucket, commitHash []byte) error {
	leafKey := bytesToRefKey(commitHash, refLeafPrefix)
	return dsBucket.Delete(leafKey)
}

// getBranchCommitRef gets the commit hash that a branch points to.
// Returns nil if the branch doesn't exist.
func getBranchCommitRef(dsBucket *bbolt.Bucket, branchName string) []byte {
	branchKey := bytesToRefKey(stringAsImmutableBytes(branchName), refBranchPrefix)
	return dsBucket.Get(branchKey)
}

// setBranchCommitRef sets the branch to point to a commit hash.
func setBranchCommitRef(dsBucket *bbolt.Bucket, branchName string, commitHash []byte) error {
	branchKey := bytesToRefKey(stringAsImmutableBytes(branchName), refBranchPrefix)
	return dsBucket.Put(branchKey, commitHash)
}

// deleteBranchCommitRef deletes a branch reference.
func deleteBranchCommitRef(dsBucket *bbolt.Bucket, branchName string) error {
	branchKey := bytesToRefKey(stringAsImmutableBytes(branchName), refBranchPrefix)
	return dsBucket.Delete(branchKey)
}

// convertBranchCommitToLeaf converts a branch commit to a leaf commit.
// It creates a leaf commit reference for the given commit and deletes the branch reference.
// This is used for:
// 1. Converting old commit to leaf when new commit has no parent (normal commit)
// 2. Converting new commit to leaf for deleted NamedGraph.
func convertBranchCommitToLeaf(dsBucket *bbolt.Bucket, branchName string, commitHash []byte) error {
	// Create leaf commit reference
	if err := createLeafCommitRef(dsBucket, commitHash); err != nil {
		return err
	}
	// Delete the branch reference
	return deleteBranchCommitRef(dsBucket, branchName)
}

// commitHasParentInBucketC checks if a commit has a parent commit in Bucket C (Commits bucket).
// It checks the dataset entry in the commit to see if there's parent commit info.
func commitHasParentInBucketC(commitsBucket *bbolt.Bucket, dsID uuid.UUID, commitHash []byte) bool {
	commitBucket := commitsBucket.Bucket(commitHash)
	if commitBucket == nil {
		return false
	}
	dsKey := iDToPrefixedKey(dsID, commitDsPrefix)
	dsValue := commitBucket.Get(dsKey)
	// dsValue contains DS-SHA (32 bytes) + parent commits
	// If length > 32, it means there's at least one parent commit
	return dsValue != nil && len(dsValue) > 32
}

// commitExistsInBucketC checks if a commit exists in Bucket C (Commits bucket).
// It validates that the commit hash references an actual commit object.
func commitExistsInBucketC(commitsBucket *bbolt.Bucket, commitHash []byte) bool {
	if commitsBucket == nil {
		return false
	}
	commitBucket := commitsBucket.Bucket(commitHash)
	return commitBucket != nil
}

// quotaState holds the per-repository quota bookkeeping fields.
// It is allocated only when the repository belongs to a SuperRepository.
type quotaState struct {
	writeMu           sync.Mutex // serializes quota-check-and-write for DocumentSet/Commit
	documentSizeBytes atomic.Int64
}

// localFullRepository and related types implement bbolt based LocalFull repository.
//
// See [storageDB] for bbolt database layout.
type localFullRepository struct {
	stateControl
	quota       *quotaState // nil when the repository is not part of a SuperRepository
	sr          SuperRepository
	url         *url.URL
	config      repoConfig
	authInfo    *sstauth.SstUserInfo
	bleveIndex  bleve.Index // getter method Bleve()
	db          *bbolt.DB
	accessRight sstauth.AccessMode // fixed at open/create time from filesystem probe
}

func (r *localFullRepository) SuperRepository() SuperRepository {
	return r.sr
}

func (r *localFullRepository) URL() string {
	return r.url.String()
}

func (r *localFullRepository) DB() *bbolt.DB {
	return r.db
}

func (r *localFullRepository) OpenStage(mode TriplexMode) Stage {
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

func (r *localFullRepository) RegisterIndexHandler(sdi *SSTDeriveInfo) error {
	if err := r.enter(); err != nil {
		return err
	}
	defer r.leave()

	blevePath := filepath.Join(r.config.repositoryDir, indexBleveDir)
	bleveInfoPath := filepath.Join(blevePath, bleveInfoName)

	if sdi.isEmpty() {
		GlobalLogger.Info("RegisterIndexHandler received empty SSTDeriveInfo, deleting bleve index")
		err := os.RemoveAll(blevePath)
		if err != nil {
			GlobalLogger.Error("failed to remove bleve index", zap.Error(err))
			return err
		}
		GlobalLogger.Info("bleve index deleted", zap.String("path", blevePath))
		return nil
	}

	r.config.deriveInfo = sdi

	// check if it already has a bleve
	bleve, err := bleve.Open(blevePath)

	// This means it has an existed bleve folder
	if err == nil {
		// Checks if the saved bleve is the same as the newly provided
		file, err := os.Open(bleveInfoPath)
		if err != nil {
			GlobalLogger.Warn("failed to open bleve info file, will rebuild index", zap.Error(err))
			bleve.Close()
			return r.buildBleveIndex()
		}
		defer file.Close()

		decoder := json.NewDecoder(file)

		// save JSON file data
		var p SSTDeriveInfo

		err = decoder.Decode(&p)
		if err != nil {
			GlobalLogger.Warn("failed to decode bleve info, will rebuild index", zap.Error(err))
			bleve.Close()
			return r.buildBleveIndex()
		}

		same := (p.DerivePackageName == sdi.DerivePackageName) && (p.DerivePackageVersion == sdi.DerivePackageVersion)
		// if same is true, use opened bleve, and only set update function
		if same {
			// set PreCommitCondition for update
			// PreCommitCondition checks for the combination of mainType + ID + IDOwner is unique for the whole repository
			var batchUpdateBatched sync.Map
			r.config.preCommitCondition = func(stage Stage) (postCommitNotifier, error) {
				pcn, err := stagePreCommitConstraints(r, bleve, &batchUpdateBatched, stage)
				if err != nil {
					return nil, err
				}
				if pcn != nil {
					return pcn, nil
				}
				return nil, nil
			}
			r.bleveIndex = newHealthAwareIndex(bleve)
			r.config.bleveIndexUpdater = newDefaultBleveIndexUpdater(r.bleveIndex.(*healthAwareIndex))
		} else {
			// if same is false, it will recreate all in index.bleve
			bleve.Close()
			if err := r.buildBleveIndex(); err != nil {
				GlobalLogger.Error("failed to rebuild bleve index", zap.Error(err))
				return fmt.Errorf("failed to rebuild bleve index: %w", err)
			}
		}
	} else {
		// If there is no index folder before, create a new one
		if err := r.buildBleveIndex(); err != nil {
			GlobalLogger.Error("failed to create bleve index", zap.Error(err))
			return fmt.Errorf("failed to create bleve index: %w", err)
		}
	}
	return nil
}

// After register, the building index work should be taken care of by caching().
// This will do all the Index work and caching it.
func (r *localFullRepository) buildBleveIndex() error {
	indexDir := filepath.Join(r.config.repositoryDir, indexBleveDir)
	bleveInfoPath := filepath.Join(indexDir, bleveInfoName)

	if r.bleveIndex != nil {
		r.bleveIndex.Close()
		r.bleveIndex = nil
	}

	// Check if the directory exists
	if _, err := os.Stat(indexDir); os.IsNotExist(err) {
		// Directory does not exist, create it
		if err := os.MkdirAll(indexDir, os.ModePerm); err != nil {
			GlobalLogger.Error("failed to create bleve index directory", zap.Error(err))
			return fmt.Errorf("failed to create bleve index directory: %w", err)
		}
		GlobalLogger.Info("bleve index directory created", zap.String("path", indexDir))
	} else if err != nil {
		GlobalLogger.Error("failed to stat bleve index directory", zap.Error(err))
		return fmt.Errorf("failed to stat bleve index directory: %w", err)
	} else {
		// Directory exists, removeAll
		GlobalLogger.Info("removing existing bleve index directory", zap.String("path", indexDir))
		if err := os.RemoveAll(indexDir); err != nil {
			GlobalLogger.Error("failed to remove old bleve index directory", zap.Error(err))
			return fmt.Errorf("failed to remove old bleve index directory: %w", err)
		}
		GlobalLogger.Info("old bleve index directory removed", zap.String("path", indexDir))
	}

	// creates a new bleve index
	bi, err := bleve.New(indexDir, r.config.deriveInfo.DefaultIndexMapping())
	if err != nil {
		GlobalLogger.Error("failed to create bleve index", zap.Error(err))
		return fmt.Errorf("failed to create bleve index: %w", err)
	}
	//  create JSON data for saving deriveInfo
	jsonData, err := json.MarshalIndent(r.config.deriveInfo, "", "    ")
	if err != nil {
		GlobalLogger.Error("failed to marshal derive info", zap.Error(err))
		bi.Close()
		return fmt.Errorf("failed to marshal derive info: %w", err)
	}

	// Create a JSON file
	file, err := os.Create(bleveInfoPath)
	if err != nil {
		GlobalLogger.Error("failed to create bleve info file", zap.Error(err))
		bi.Close()
		return fmt.Errorf("failed to create bleve info file: %w", err)
	}
	defer file.Close()

	// put data into JSON file
	_, err = file.Write(jsonData)
	if err != nil {
		GlobalLogger.Error("failed to write bleve info file", zap.Error(err))
		bi.Close()
		return fmt.Errorf("failed to write bleve info file: %w", err)
	}

	// set PreCommitCondition for update
	var batchUpdateBatched sync.Map
	r.config.preCommitCondition = func(stage Stage) (postCommitNotifier, error) {
		return stagePreCommitConstraints(r, bi, &batchUpdateBatched, stage)
	}

	r.bleveIndex = newHealthAwareIndex(bi)

	// Use the default asynchronous Bleve updater.
	r.config.bleveIndexUpdater = newDefaultBleveIndexUpdater(r.bleveIndex.(*healthAwareIndex))

	if err := r.view(func(rFS fs.FS) error {
		return rebuildIndex(context.Background(), r, bi, rFS)
	}); err != nil {
		GlobalLogger.Error("failed to rebuild index", zap.Error(err))
		bi.Close()
		return fmt.Errorf("failed to rebuild index: %w", err)
	}

	return nil
}

func (r *localFullRepository) Bleve() bleve.Index {
	if err := r.enter(); err != nil {
		GlobalLogger.Error("failed to get bleve", zap.Error(err))
		return nil
	}
	defer r.leave()

	return r.bleveIndex
}

// RebuildBleveIndex closes the current index, removes the index directory,
// and rebuilds it from scratch. The repository remains usable for BBolt
// operations during the rebuild, but search is unavailable.
func (r *localFullRepository) RebuildBleveIndex() error {
	if err := r.enter(); err != nil {
		return err
	}
	defer r.leave()

	if r.config.bleveIndexUpdater != nil {
		r.config.bleveIndexUpdater.close()
		r.config.bleveIndexUpdater = nil
	}

	if r.bleveIndex != nil {
		r.bleveIndex.Close()
		r.bleveIndex = nil
	}

	return r.buildBleveIndex()
}

func (r *localFullRepository) DatasetIDs(ctx context.Context) ([]uuid.UUID, error) {
	if err := r.enter(); err != nil {
		return nil, wrapError(err)
	}
	defer r.leave()

	var datasets []uuid.UUID

	err := r.db.View(func(tx *bbolt.Tx) error {
		datasetsBucket := tx.Bucket(keyDatasets)
		if datasetsBucket == nil {
			return fmt.Errorf("datasets bucket not found")
		}

		return datasetsBucket.ForEach(func(k, v []byte) error {
			id, err := uuid.FromBytes(k)
			if err != nil {
				return fmt.Errorf("invalid dataset ID: %w", err)
			}
			datasets = append(datasets, id)
			return nil
		})
	})
	if err != nil {
		return nil, wrapError(err)
	}

	return datasets, nil
}

func (r *localFullRepository) Datasets(ctx context.Context) ([]IRI, error) {
	if err := r.enter(); err != nil {
		return nil, wrapError(err)
	}
	defer r.leave()

	var datasetIRIs []IRI

	err := r.db.View(func(tx *bbolt.Tx) error {
		datasets := tx.Bucket(keyDatasets)
		if datasets == nil {
			return nil
		}
		c := datasets.Cursor()

		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			dsBucket := datasets.Bucket(k)
			dsID := uuid.UUID(*(*[len(uuid.UUID{})]byte)(k))

			// Use getDatasetIRI to get the IRI from the dataset bucket
			dsIRI := getDatasetIRI(dsBucket, dsID)
			datasetIRIs = append(datasetIRIs, IRI(strings.TrimSuffix(dsIRI, "#")))
		}
		return nil
	})
	if err != nil {
		return nil, wrapError(err)
	}
	return datasetIRIs, nil
}

func (r *localFullRepository) ForDatasets(ctx context.Context, c func(ds Dataset) error) error {
	if err := r.enter(); err != nil {
		return wrapError(err)
	}
	defer r.leave()

	return r.db.View(func(tx *bbolt.Tx) error {
		datasets := tx.Bucket(keyDatasets)
		if datasets == nil {
			return nil
		}
		cursor := datasets.Cursor()

		for k, _ := cursor.First(); k != nil; k, _ = cursor.Next() {
			dsBucket := datasets.Bucket(k)
			dsID := uuid.UUID(*(*[len(uuid.UUID{})]byte)(k))

			// Use getDatasetIRI to get the IRI from the dataset bucket
			dsIRI := getDatasetIRI(dsBucket, dsID)
			ds := &localFullDataset{r: r, id: dsID, iri: IRI(dsIRI)}

			if err := c(ds); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *localFullRepository) datasetByID(ctx context.Context, id uuid.UUID) (di Dataset, err error) {
	if err := r.enter(); err != nil {
		return nil, wrapError(err)
	}
	defer r.leave()

	storedIRI := ""

	err = r.db.View(func(tx *bbolt.Tx) error {
		if datasets := tx.Bucket(keyDatasets); datasets != nil {
			actualID, _ := datasets.Cursor().Seek(id[:])
			if !bytes.Equal(actualID, id[:]) {
				return ErrDatasetNotFound
			}
			// Get the stored IRI for this dataset
			dsBucket := datasets.Bucket(id[:])
			if dsBucket != nil {
				storedIRI = getDatasetIRI(dsBucket, id)
				if storedIRI == "" {
					// If no stored IRI found, fall back to URN format
					storedIRI = id.URN()
				}
			} else {
				storedIRI = id.URN()
			}
			return nil
		}

		return ErrDatasetNotFound
	})
	if err != nil {
		return
	}

	return &localFullDataset{r: r, id: id, iri: IRI(storedIRI)}, nil
}

func (r *localFullRepository) Dataset(ctx context.Context, iri IRI) (di Dataset, err error) {
	id, err := iriToUUID(iri)
	if err != nil {
		return nil, err
	}
	return r.datasetByID(ctx, id)
}

func (r *localFullRepository) CommitDetails(ctx context.Context, hashes []Hash) ([]*CommitDetails, error) {
	if err := r.enter(); err != nil {
		return nil, wrapError(err)
	}
	defer r.leave()

	var results []*CommitDetails

	err := r.db.View(func(tx *bbolt.Tx) error {
		commitsBucket := tx.Bucket(keyCommits)
		if commitsBucket == nil {
			return fmt.Errorf("commits bucket not found")
		}

		for _, hash := range hashes {
			commitBucket := commitsBucket.Bucket(hash[:])
			if commitBucket == nil {
				// If commit not found, skip
				continue
			}

			details := &CommitDetails{
				Commit:              hash,
				Author:              "",
				AuthorDate:          time.Time{},
				Message:             "",
				ParentCommits:       map[IRI][]Hash{},
				DatasetRevisions:    map[IRI]Hash{},
				NamedGraphRevisions: map[IRI]Hash{},
			}
			datasets := tx.Bucket(keyDatasets)

			// 1. Author
			if authorData := commitBucket.Get(commitKeyAuthor); len(authorData) > 8 {
				details.Author = string(authorData[:len(authorData)-8])
				ts := binary.BigEndian.Uint64(authorData[len(authorData)-8:])
				details.AuthorDate = time.Unix(int64(ts), 0).UTC()
			}

			// 2. Message
			if msg := commitBucket.Get(commitKeyMessage); msg != nil {
				details.Message = string(msg)
			}

			// 3. DatasetRevisions and ParentCommits
			_ = commitBucket.ForEach(func(k, v []byte) error {
				if len(k) != 17 || k[0] != commitDsPrefix || len(v) < 32 {
					return nil
				}
				var dsID uuid.UUID
				copy(dsID[:], k[1:])
				// Get IRI for this dataset
				dsIRI := IRI(dsID.URN())
				if datasets != nil {
					if dsBucket := datasets.Bucket(k[1:]); dsBucket != nil {
						dsIRI = IRI(getDatasetIRI(dsBucket, dsID))
					}
				}

				var dsHash Hash
				copy(dsHash[:], v[:32])
				details.DatasetRevisions[dsIRI] = dsHash

				// parse all parent commits (if any)
				extra := v[32:]
				for i := 0; i+32 <= len(extra); i += 32 {
					var parent Hash
					copy(parent[:], extra[i:i+32])
					details.ParentCommits[dsIRI] = append(details.ParentCommits[dsIRI], parent)
				}
				return nil
			})

			// 4. NamedGraphRevisions
			dsrTop := tx.Bucket(keyDatasetRevisions)
			if dsrTop != nil {
				for dsIRI, dsrHash := range details.DatasetRevisions {
					dsrBucket := dsrTop.Bucket(dsrHash[:])
					if dsrBucket == nil {
						continue
					}

					// 4.1 Default Named Graph (key = 0x00)
					// The default Named Graph UUID is the same as the Dataset UUID
					if v := dsrBucket.Get([]byte{0x00}); len(v) == 32 {
						var ngr Hash
						copy(ngr[:], v)
						if ngr != (Hash{}) {
							details.NamedGraphRevisions[dsIRI] = ngr
						}
					}

					// 4.2 Imported Named Graphs (key = ngPrefix + NG_UUID)
					_ = dsrBucket.ForEach(func(k, v []byte) error {
						if len(k) == 17 && k[0] == ngPrefix && len(v) == 32 {
							var ngID uuid.UUID
							copy(ngID[:], k[1:])
							ngIRI := IRI(ngID.URN())
							if datasets != nil {
								if ngBucket := datasets.Bucket(k[1:]); ngBucket != nil {
									ngIRI = IRI(getDatasetIRI(ngBucket, ngID))
								}
							}
							var ngr Hash
							copy(ngr[:], v)
							if ngr != (Hash{}) {
								details.NamedGraphRevisions[ngIRI] = ngr
							}
						}
						// Note: dsPrefix + UUID represents the DSR of an imported Dataset, ignored here
						return nil
					})
				}
			}

			results = append(results, details)
		}

		return nil
	})
	if err != nil {
		return results, wrapError(err)
	}
	return results, nil
}

func (r *localFullRepository) CheckoutCommit(ctx context.Context, commitID Hash, mode TriplexMode) (Stage, error) {
	if err := r.enter(); err != nil {
		return nil, wrapError(err)
	}
	defer r.leave()

	cds, err := r.CommitDetails(ctx, []Hash{commitID})
	if err != nil || len(cds) == 0 || cds[0] == nil {
		return nil, wrapError(ErrCommitNotFound)
	}
	return checkoutCommitFromCommitDetails(ctx, r, commitID, mode, cds[0])
}

func (r *localFullRepository) Close() error {
	// mark stateClosing - CompareAndSwap will make sure only marked once
	if !r.state.CompareAndSwap(int32(stateOpen), int32(stateClosing)) {
		// Already closing or closed
		return nil
	}

	// Wait for all operations that have been entered to exit
	r.wg.Wait()

	// Close the Bleve index updater so the background worker drains
	// before the Bleve index itself is closed.
	if r.config.bleveIndexUpdater != nil {
		r.config.bleveIndexUpdater.close()
	}

	// Close the bleveIndex and bbolt
	if r.bleveIndex != nil {
		r.bleveIndex.Close()
	}
	if err := r.db.Close(); err != nil {
		return err
	}

	r.state.Store(int32(stateClosed))

	return nil
}

func (r *localFullRepository) preCommitCondition() preCommitConditionCallback {
	return r.config.preCommitCondition
}

func (r *localFullRepository) postCommitNotification() postCommitNotificationCallback {
	return r.config.postCommitNotification
}

type localFullDataset struct {
	r   *localFullRepository
	id  uuid.UUID
	iri IRI
}

func (d *localFullDataset) FindCommonParentRevision(ctx context.Context, revision1, revision2 Hash) (parent Hash, err error) {
	if err := d.r.enter(); err != nil {
		return emptyHash, wrapError(err)
	}
	defer d.r.leave()

	if revision1 == revision2 {
		return HashNil(), ErrSameRevisions
	}

	// Double pointer to find common parent
	// Two pointers walk backward through commit history
	// When they hit the end, they jump to the other commit's start
	// If they meet → found common parent
	// If pCd1 hits the end twice → no common commit
	commitDetail1, err := d.CommitDetailsByHash(ctx, revision1)
	if err != nil {
		GlobalLogger.Error("CommitDetailsByHash error", zap.Error(err))
		return HashNil(), err
	}

	commitDetail2, err := d.CommitDetailsByHash(ctx, revision2)
	if err != nil {
		GlobalLogger.Error("CommitDetailsByHash error", zap.Error(err))
		return HashNil(), err
	}

	if len(commitDetail1.ParentCommits[d.iri]) == 0 && len(commitDetail2.ParentCommits[d.iri]) == 0 {
		return HashNil(), nil
	}

	pCd1 := commitDetail1
	pCd2 := commitDetail2
	noCommonCommit := false
	loopTimes := 0

	for pCd1.Commit != pCd2.Commit {
		if len(pCd1.ParentCommits[d.iri]) == 0 {
			if loopTimes == 1 {
				noCommonCommit = true
				break
			}
			loopTimes++
			pCd1 = commitDetail2
		} else {
			pCd1, err = d.CommitDetailsByHash(ctx, pCd1.ParentCommits[d.iri][0])
			if err != nil {
				GlobalLogger.Error("CommitDetailsByHash error", zap.Error(err))
				return HashNil(), err
			}
		}

		if len(pCd2.ParentCommits[d.iri]) == 0 {
			pCd2 = commitDetail1
		} else {
			pCd2, err = d.CommitDetailsByHash(ctx, pCd2.ParentCommits[d.iri][0])
			if err != nil {
				GlobalLogger.Error("CommitDetailsByHash error", zap.Error(err))
				return HashNil(), err
			}
		}
	}

	if noCommonCommit {
		return HashNil(), nil
	} else {
		return pCd1.Commit, nil
	}
}

func (d *localFullDataset) IRI() IRI {
	return d.iri
}

func (d *localFullDataset) SetBranchCommit(ctx context.Context, commit Hash, branch string) error {
	if err := d.r.enter(); err != nil {
		return wrapError(err)
	}
	defer d.r.leave()

	err := d.r.db.Update(func(tx *bbolt.Tx) error {
		bucketDs, err := tx.CreateBucketIfNotExists(keyDatasets)
		if err != nil {
			GlobalLogger.Error("", zap.Error(err))
			return err
		}

		specifiedDsBucket, err := createDatasetBucketIfNotExists(bucketDs, d.id)
		if err != nil {
			GlobalLogger.Error("", zap.Error(err))
			return err
		}

		// looking for if the commit exists in the dataset
		found := false
		isLeafCommit := false
		err = specifiedDsBucket.ForEach(func(k, v []byte) error {
			if bytes.Equal(v, commit[:]) {
				found = true
				if bytes.Equal(k, bytesToRefKey(commit[:], refLeafPrefix)) {
					isLeafCommit = true
				}
				return nil
			}
			return nil
		})
		if err != nil {
			return err
		}

		// Set branch if commit found
		if found {
			if err := specifiedDsBucket.Put(bytesToRefKey(stringAsImmutableBytes(branch), refBranchPrefix), commit[:]); err != nil {
				GlobalLogger.Error("", zap.Error(err))
				return err
			}
			// due to leaf commit being moved to a branch, delete the leaf ref
			// this is not needed if the commit is already been pointed by a branch
			// because there could be multiple branches pointing to the same commit
			if isLeafCommit {
				err = specifiedDsBucket.Delete(bytesToRefKey(commit[:], refLeafPrefix))
				if err != nil {
					return err
				}
			}
		} else {
			// return error if commit not found in dataset, so it cannot be set to branch
			return fmt.Errorf("failed to set branch, commit %s not found in dataset %s", commit.String(), d.id.String())
		}

		// Write log entry
		repoLogBucket, err := tx.CreateBucketIfNotExists([]byte("log"))
		if err != nil {
			return err
		}

		cd, err := d.CommitDetailsByHash(ctx, commit)
		if err != nil {
			return err
		}

		revisionHash := ""
		if cd != nil {
			if revHash, ok := cd.DatasetRevisions[d.iri]; ok {
				revisionHash = revHash.String()
			}
		}

		email := "default@semanticstep.net"
		if u := sstauth.SstUserInfoFromContext(ctx); u != nil && u.Email != "" {
			email = u.Email
		}

		fields := map[string]string{
			"type":        "set_branch",
			"message":     "set branch",
			"author":      email,
			"timestamp":   time.Now().UTC().Format(time.RFC3339),
			"dataset":     d.id.String(),
			"branch":      branch,
			"ds_revision": revisionHash,
		}

		return writeRepositoryLogEntry(repoLogBucket, fields)
	})
	if err != nil {
		return wrapError(err)
	}

	if d.r.config.bleveIndexUpdater != nil {
		d.r.config.bleveIndexUpdater.updateAfterSetBranch(ctx, d.r, d.id, branch)
	}

	return nil
}

func (d *localFullDataset) SetBranchRevision(ctx context.Context, datasetRevision Hash, branch string) error {
	commit, err := d.CommitForRevision(ctx, datasetRevision)
	if err != nil {
		return wrapError(err)
	}
	return d.SetBranchCommit(ctx, commit, branch)
}

func (d *localFullDataset) RemoveBranch(ctx context.Context, branch string) error {
	if err := d.r.enter(); err != nil {
		return wrapError(err)
	}
	defer d.r.leave()

	err := d.r.db.Update(func(tx *bbolt.Tx) error {
		bucketDs, err := tx.CreateBucketIfNotExists(keyDatasets)
		if err != nil {
			GlobalLogger.Error("", zap.Error(err))
			return err
		}

		newDsBucket, err := createDatasetBucketIfNotExists(bucketDs, d.id)
		if err != nil {
			GlobalLogger.Error("", zap.Error(err))
			return err
		}

		refKey := bytesToRefKey(stringAsImmutableBytes(branch), refBranchPrefix)
		commitBytes := newDsBucket.Get(refKey)
		if commitBytes == nil {
			return fmt.Errorf("branch %s not found in dataset %s", branch, d.id.String())
		}

		revisionHash := ""
		if len(commitBytes) == len(Hash{}) {
			var commit Hash
			copy(commit[:], commitBytes)

			if cd, err := d.CommitDetailsByHash(ctx, commit); err == nil && cd != nil {
				if revHash, ok := cd.DatasetRevisions[d.iri]; ok {
					revisionHash = revHash.String()
				}
			}
		}

		// branch commit is deleted, this commit will become a leaf commit when this is no branch pointing to it
		if err := newDsBucket.Delete(refKey); err != nil {
			GlobalLogger.Error("", zap.Error(err))
			return err
		}

		// looking for if the commit is pointed by other branches
		found := false
		isBranchCommit := false
		err = newDsBucket.ForEach(func(k, v []byte) error {
			if bytes.Equal(v, commitBytes) {
				found = true
				if bytes.HasPrefix(k, []byte{byte(refBranchPrefix)}) {
					isBranchCommit = true
				}
				return nil
			}
			return nil
		})
		if err != nil {
			return err
		}

		// if found means this commit is still stored in the dataset
		// maybe as a branch commit or leaf commit
		// if this is a branch commit, do nothing
		// if this is not a branch commit, means no branch is pointing to it, this should not happen
		if found {
			if isBranchCommit {
				// do nothing, as other branches are still pointing to this commit
			} else {
				GlobalLogger.Error("commit is still stored as leaf commit, but no branch is pointing to it, this should not happen",
					zap.String("dataset", d.id.String()), zap.String("branch", branch), zap.String("commit", BytesToHash(commitBytes).String()))
			}
		} else {
			// else if not found, means no branch is pointing to this commit anymore
			// add it as a leaf commit
			if err := newDsBucket.Put(bytesToRefKey(commitBytes, refLeafPrefix), commitBytes); err != nil {
				GlobalLogger.Error("", zap.Error(err))
				return err
			}
		}

		// Write log entry
		repoLogBucket, err := tx.CreateBucketIfNotExists([]byte("log"))
		if err != nil {
			return err
		}

		email := "default@semanticstep.net"
		if u := sstauth.SstUserInfoFromContext(ctx); u != nil && u.Email != "" {
			email = u.Email
		}

		fields := map[string]string{
			"type":        "remove_branch",
			"message":     "remove branch",
			"author":      email,
			"timestamp":   time.Now().UTC().Format(time.RFC3339),
			"dataset":     d.id.String(),
			"branch":      branch,
			"ds_revision": revisionHash,
		}

		return writeRepositoryLogEntry(repoLogBucket, fields)
	})
	if err != nil {
		return wrapError(err)
	}

	if d.r.config.bleveIndexUpdater != nil {
		d.r.config.bleveIndexUpdater.updateAfterRemoveBranch(ctx, d.r, d.id, branch)
	}

	return nil
}

func (d *localFullDataset) ID() uuid.UUID {
	return d.id
}

func (d *localFullDataset) Branches(ctx context.Context) (map[string]Hash, error) {
	if err := d.r.enter(); err != nil {
		return nil, wrapError(err)
	}
	defer d.r.leave()

	branchCommitHashMap := make(map[string]Hash)
	err := d.r.db.Update(func(tx *bbolt.Tx) error {
		bucketDs, err := tx.CreateBucketIfNotExists(keyDatasets)
		if err != nil {
			GlobalLogger.Error("", zap.Error(err))
			return err
		}

		newDsBucket, err := createDatasetBucketIfNotExists(bucketDs, d.id)
		if err != nil {
			GlobalLogger.Error("", zap.Error(err))
			return err
		}

		newDsBucket.ForEach(func(k, v []byte) error {
			if len(k) > 0 && k[0] == '\x00' {
				// fmt.Printf("%s branch: %s latest _Commit_SHA: %x\n", indent, k[1:], v)
				branchCommitHashMap[string(k[1:])] = BytesToHash(v)
			} else if len(k) > 0 && k[0] == '\x01' {
				// fmt.Printf("%stag name: %s latest _Commit_SHA: %x\n", indent, k[1:], v)
			} else if len(k) > 0 && k[0] == '\x02' {
				// fmt.Printf("%s_Commit_SHA_: %s latest _Commit_SHA: %x\n", indent, k[1:], v)
			} else {
				// fmt.Printf("%sKey: %s, Value: %x\n", indent, k, v)
				GlobalLogger.Error("unknown value in Dataset Bucket", zap.String("key", string(k)))
				return errUnknownValue
			}
			return nil
		})

		return nil
	})
	if err != nil {
		return branchCommitHashMap, wrapError(err)
	}
	return branchCommitHashMap, nil
}

func (d *localFullDataset) LeafCommits(ctx context.Context) ([]Hash, error) {
	if err := d.r.enter(); err != nil {
		return nil, wrapError(err)
	}
	defer d.r.leave()

	var returnHashes []Hash

	err := d.r.db.View(func(tx *bbolt.Tx) error {
		bucketDs := tx.Bucket(keyDatasets)
		if bucketDs == nil {
			GlobalLogger.Error("datasets bucket not found", zap.Error(errBucketNotExist))
			return errBucketNotExist
		}

		currentDsBucket := bucketDs.Bucket(d.id[:])
		if currentDsBucket == nil {
			GlobalLogger.Error("dataset not found in bucket", zap.Error(errBucketNotExist))
			return errBucketNotExist
		}
		// only return leaf commits that means key prefix is refLeafPrefix
		return currentDsBucket.ForEach(func(k, v []byte) error {
			if len(k) > 0 && k[0] == byte(refBranchPrefix) {
				// returnHashes = append(returnHashes, BytesToHash(v))
			} else if len(k) > 0 && k[0] == byte(refLeafPrefix) {
				returnHashes = append(returnHashes, BytesToHash(v))
			} else {
				GlobalLogger.Error("unknown value in Dataset Bucket when getting LeafCommits", zap.String("key", string(k)))
				return errUnknownValue
			}
			return nil
		})
	})
	if err != nil {
		return nil, wrapError(err)
	}

	return returnHashes, nil
}

func (d *localFullDataset) AssertStage(Stage) {}

func checkoutCommon(ds *localFullDataset, commitHash Hash) (Stage, error) {
	var dsRevision Hash
	err := ds.r.db.View(func(tx *bbolt.Tx) error {
		bucketC := tx.Bucket(keyCommits)
		if bucketC == nil {
			GlobalLogger.Error("commits bucket not found", zap.Error(errBucketNotExist))
			return errBucketNotExist
		}

		// find specific commit as sub-bucket
		subBucketC := bucketC.Bucket(commitHash[:])
		if subBucketC == nil {
			GlobalLogger.Error("commit bucket not found", zap.Error(errBucketNotExist))
			return errBucketNotExist
		}

		// find specific Dataset Revision
		dsKey := iDToPrefixedKey(ds.id, commitDsPrefix)
		actualDSKey, dsHash := subBucketC.Cursor().Seek(dsKey)
		if !bytes.Equal(actualDSKey, dsKey) {
			GlobalLogger.Error("DatasetRevision not found", zap.Error(errBucketNotExist))
			return errDatasetRevisionMissing
		}

		dsRevision = BytesToHash(dsHash[:len(hashT{})])
		return nil
	})
	if err != nil {
		return nil, wrapError(err)
	}
	return checkoutRevisionCommon(ds, dsRevision, commitHash)
}

func checkoutRevisionCommon(ds *localFullDataset, dsRevision Hash, commitHash Hash) (Stage, error) {
	var st *stage

	err := ds.r.db.View(func(tx *bbolt.Tx) error {
		var subBucketDSR *bbolt.Bucket
		bucketDSR := tx.Bucket(keyDatasetRevisions)

		// find specific DSR as sub-bucket
		subBucketDSR = bucketDSR.Bucket(dsRevision[:])
		if subBucketDSR == nil {
			GlobalLogger.Error("DatasetRevision not found", zap.Error(errDatasetRevisionMissing))
			return errDatasetRevisionMissing
		}

		// result: all imported dataset UUID -> imported DS-SHA
		importedDsIDAndDsrHash := make(map[uuid.UUID]Hash)

		// visited revisions by DS-SHA to avoid cycles and repeated work
		visitedSHA := make(map[Hash]struct{})

		var walk func(dsSHA Hash) error
		walk = func(dsSHA Hash) error {
			if _, ok := visitedSHA[dsSHA]; ok {
				return nil
			}
			visitedSHA[dsSHA] = struct{}{}

			subDsrBucket := bucketDSR.Bucket(dsSHA[:])
			if subDsrBucket == nil {
				// Depending on your needs: treat as hard error or just ignore missing nodes.
				return fmt.Errorf("dsr subBucket not found for DS-SHA %q", dsSHA)
			}

			// Extract all direct imports from this subBucket.
			imports := make(map[uuid.UUID]Hash)
			subDsrBucket.ForEach(func(k, v []byte) error {
				if len(k) > 1 && k[0] == '\x00' {
					if uuid.UUID(k[1:]) != ds.id {
						imports[uuid.UUID(k[1:])] = BytesToHash(v)
					}
				}
				return nil
			})

			for id, sha := range imports {
				// record result
				if prev, exists := importedDsIDAndDsrHash[id]; exists && prev != sha {
					// conflict: same dataset id points to different sha via different paths
					// choose to keep first; you can also return error if you want strictness
					// return fmt.Errorf("conflict: imported dataset %s has multiple DS-SHA: %q vs %q", id, prev, sha)
				} else {
					importedDsIDAndDsrHash[id] = sha
				}

				// recurse
				if err := walk(sha); err != nil {
					return err
				}
			}
			return nil
		}

		err := walk(dsRevision)
		if err != nil {
			return err
		}

		// save imported NG ID and Hash into a map
		importedNGAndHash := make(map[uuid.UUID]Hash)
		subBucketDSR.ForEach(func(k, v []byte) error {
			if len(k) > 0 && k[0] == '\x01' {
				// fmt.Printf("NG-UUID:%x\n", k[1:])
				if uuid.UUID(k[1:]) != ds.id {
					importedNGAndHash[uuid.UUID(k[1:])] = BytesToHash(v)
				}
				// fmt.Printf("NG-SHA :%x\n", v)
			}
			return nil
		})

		// find dataset default NG
		// nameKeySeek := IDToPrefixedKey(d.id, NgPrefix)
		tempKey, ngHash := subBucketDSR.Cursor().Seek([]byte{0x00})
		if tempKey == nil {
			GlobalLogger.Error("NamedGraph not found", zap.Error(errNamedGraphNotFoundInDataset))
			return errNamedGraphNotFoundInDataset
		}

		// means this NamedGraph is deleted
		if bytes.Equal(ngHash, emptyHash[:]) {
			return ErrDatasetHasBeenDeleted
		}

		var ngActualHash []byte
		var ngContent []byte
		if !bytes.Equal(ngHash, emptyHash[:]) {
			// find NG content from bucketNGR
			ngActualHash, ngContent = tx.Bucket(keyNamedGraphRevisions).Cursor().Seek(ngHash)
			if !bytes.Equal(ngHash, ngActualHash) {
				GlobalLogger.Error("NamedGraphRevision not found", zap.Error(errNamedGraphRevisionNotFound))
				return errNamedGraphRevisionNotFound
			}
		}
		dgFile := byteFileReader{keyedFile: keyedFile{ds.id}, byteReader: bytes.NewReader(ngContent)}
		defer dgFile.Close()

		ng, err := SstRead(bufio.NewReader(dgFile), DefaultTriplexMode)
		if err != nil {
			GlobalLogger.Error("", zap.Error(err))
			return err
		}
		st = ng.Stage().(*stage)
		st.repo = ds.r

		defaultNG := st.localGraphs[ds.id]
		if commitHash != emptyHash {
			defaultNG.checkedOutCommits = []Hash{commitHash}
		} else {
			// Look up commit hash from dataset revision
			commitHashBytes := subBucketDSR.Get([]byte{0x02})
			if commitHashBytes != nil {
				defaultNG.checkedOutCommits = []Hash{BytesToHash(commitHashBytes)}
			}
		}
		defaultNG.checkedOutDSRevisions = []Hash{dsRevision}
		defaultNG.checkedOutNGRevisions = []Hash{Hash(ngHash)}
		// defaultNG.flags.modified = false

		// handle imported NGs
		var importedNGActualHash []byte
		var importedNGContent []byte
		for importedNGID, importedNGHash := range importedNGAndHash {
			if importedNGHash == emptyHash {
				continue
			}
			// find importedNGContent from bucketNGR
			importedNGActualHash, importedNGContent = tx.Bucket(keyNamedGraphRevisions).Cursor().Seek(importedNGHash[:])
			if importedNGHash != BytesToHash(importedNGActualHash) {
				GlobalLogger.Error("NamedGraphRevision not found", zap.Error(errNamedGraphRevisionNotFound))
				return errNamedGraphRevisionNotFound
			}
			dgFile := byteFileReader{keyedFile: keyedFile{importedNGID}, byteReader: bytes.NewReader(importedNGContent)}
			defer dgFile.Close()

			ng, err := SstRead(bufio.NewReader(dgFile), DefaultTriplexMode)
			if err != nil {
				GlobalLogger.Error("", zap.Error(err))
				return err
			}
			_, err = st.MoveAndMerge(context.TODO(), ng.Stage())
			if err != nil {
				GlobalLogger.Error("", zap.Error(err))
				return err
			}
			importedNG := st.localGraphs[importedNGID]

			// look for the commitHash of imported NG
			importedDatasetRevisionHash, foundImportedDatasetRevision := importedDsIDAndDsrHash[importedNGID]
			if !foundImportedDatasetRevision {
				GlobalLogger.Error("DatasetRevision not found for imported NG", zap.String("importedNGID", importedNGID.String()))
				// return errDatasetRevisionMissing
			}

			var importedNGCommitHash []byte
			commitHash, err := findCommitHashForDatasetRevision(tx, importedNGID, importedDatasetRevisionHash)
			if err != nil {
				if err == ErrDatasetRevisionNotFound {
					GlobalLogger.Error("DatasetRevision not found for imported NG", zap.Error(errDatasetRevisionMissing))
				} else {
					GlobalLogger.Error("findCommitHashForDatasetRevision error", zap.Error(err))
				}
				// return errDatasetRevisionMissing
			} else {
				importedNGCommitHash = commitHash[:]
				GlobalLogger.Info("imported NG commit hash found", zap.String("importedNGID", importedNGID.String()),
					zap.String("importedDatasetRevisionHash", importedDatasetRevisionHash.String()),
					zap.String("importedNGCommitHash", commitHash.String()))
			}

			importedNG.checkedOutCommits = []Hash{BytesToHash(importedNGCommitHash)}
			importedNG.checkedOutDSRevisions = []Hash{importedDatasetRevisionHash}
			importedNG.checkedOutNGRevisions = []Hash{importedNGHash}
			// importedNG.flags.modified = false
		}
		for _, ng := range st.localGraphs {
			ng.flags.modified = false
		}

		return nil
	})
	if err != nil {
		return st, wrapError(err)
	}
	return st, nil
}

func (d *localFullDataset) CheckoutBranch(ctx context.Context, br string, mode TriplexMode) (Stage, error) {
	if err := d.r.enter(); err != nil {
		return nil, wrapError(err)
	}
	defer d.r.leave()

	commitHash, err := branchToCommit(d.r, d.id, br)
	if err != nil {
		return nil, wrapError(err)
	}

	s, err := checkoutCommon(d, commitHash)
	if err != nil {
		if err == ErrDatasetHasBeenDeleted {
			return nil, err
		} else {
			GlobalLogger.Error("", zap.Error(err))
			return nil, wrapError(err)
		}
	}

	return s, nil
}

func (d *localFullDataset) Repository() Repository {
	if err := d.r.enter(); err != nil {
		GlobalLogger.Error("failed to get Repository", zap.Error(err))
		return nil
	}
	defer d.r.leave()

	return d.r
}

func (d *localFullDataset) CheckoutCommit(ctx context.Context, commitID Hash, mode TriplexMode) (Stage, error) {
	if err := d.r.enter(); err != nil {
		return nil, wrapError(err)
	}
	defer d.r.leave()

	s, err := checkoutCommon(d, commitID)
	if err != nil {
		if err == ErrDatasetHasBeenDeleted {
			return nil, err
		} else {
			GlobalLogger.Error("", zap.Error(err))
			return nil, wrapError(err)
		}
	}

	return s, nil
}

func (d *localFullDataset) CheckoutRevision(ctx context.Context, datasetRevision Hash, mode TriplexMode) (Stage, error) {
	if err := d.r.enter(); err != nil {
		return nil, wrapError(err)
	}
	defer d.r.leave()

	s, err := checkoutRevisionCommon(d, datasetRevision, emptyHash)
	if err != nil {
		if err == ErrDatasetHasBeenDeleted {
			return nil, err
		} else {
			GlobalLogger.Error("", zap.Error(err))
			return nil, wrapError(err)
		}
	}

	return s, nil
}

func (d *localFullDataset) CommitDetailsByHash(ctx context.Context, commitID Hash) (*CommitDetails, error) {
	if err := d.r.enter(); err != nil {
		return nil, wrapError(err)
	}
	defer d.r.leave()

	cds, err := d.r.CommitDetails(ctx, []Hash{commitID})
	if err != nil {
		return nil, wrapError(err)
	}

	return cds[0], nil
}

func branchToCommit(r *localFullRepository, dsID uuid.UUID, branch string) (Hash, error) {
	var commitHashByte []byte

	err := r.db.View(func(tx *bbolt.Tx) error {
		if datasets := tx.Bucket(keyDatasets); datasets != nil {
			dsRefs := datasets.Bucket(dsID[:])
			if dsRefs == nil {
				return ErrDatasetNotFound
			}
			branchKey := bytesToRefKey([]byte(branch), refBranchPrefix)
			var actualBranchKey []byte
			actualBranchKey, commitHashByte = dsRefs.Cursor().Seek(branchKey)
			if !bytes.Equal(actualBranchKey, branchKey) {
				return ErrBranchNotFound
			}
			return nil
		}
		return ErrDatasetNotFound
	})
	if err != nil {
		return HashNil(), err
	}

	return BytesToHash(commitHashByte), nil
}

func (d *localFullDataset) CommitDetailsByBranch(ctx context.Context, branch string) (*CommitDetails, error) {
	if err := d.r.enter(); err != nil {
		return nil, wrapError(err)
	}
	defer d.r.leave()

	commitHash, err := branchToCommit(d.r, d.id, branch)
	if err != nil {
		return nil, wrapError(err)
	}

	return d.CommitDetailsByHash(ctx, commitHash)
}

func (d *localFullDataset) CommitForRevision(ctx context.Context, datasetRevision Hash) (Hash, error) {
	if err := d.r.enter(); err != nil {
		return HashNil(), wrapError(err)
	}
	defer d.r.leave()

	var commitHash Hash
	err := d.r.db.View(func(tx *bbolt.Tx) error {
		var err error
		commitHash, err = findCommitHashForDatasetRevision(tx, d.id, datasetRevision)
		return err
	})
	if err != nil {
		return HashNil(), wrapError(err)
	}
	return commitHash, nil
}

func findCommitHashForDatasetRevision(tx *bbolt.Tx, dsID uuid.UUID, dsRevision Hash) (Hash, error) {
	subDsr := tx.Bucket(keyDatasetRevisions).Bucket(dsRevision[:])
	if subDsr != nil {
		commitHashBytes := subDsr.Get([]byte{0x02})
		if commitHashBytes != nil {
			return BytesToHash(commitHashBytes), nil
		}
	}

	// Fallback: scan all commits to find the one containing this dataset revision.
	var commitHash Hash
	var found bool

	err := tx.Bucket(keyCommits).ForEach(func(k, v []byte) error {
		// In bbolt, if v==nil, k is a subBucket name.
		if v != nil {
			return nil
		}
		cb := tx.Bucket(keyCommits).Bucket(k) // commit subBucket
		if cb == nil {
			return nil
		}

		_ = cb.ForEach(func(kk, vv []byte) error {
			if len(kk) > 0 && kk[0] == '\x00' {
				if bytes.Equal(kk[1:], dsID[:]) {
					if len(vv) >= 32 && bytes.Equal(vv[:32], dsRevision[:]) {
						commitHash = BytesToHash(k) // k is commit SHA
						found = true
						return nil
					}
				}
			}
			return nil
		})

		return nil
	})
	if err != nil {
		return HashNil(), err
	}

	if !found {
		return HashNil(), ErrDatasetRevisionNotFound
	}

	return commitHash, nil
}

// bucket manipulation for commit:
//  1. calculate NamedGraph-Revision-Hash for each modified NG and put into bucketNGR.
//     bucketNGR (NamedGraph Revision): key: NG-Revision-Hash, value: sst file content
//     for each modified NG, update its stage.localGraphs[id].checkedOutNGRevisions to the new NG-Revision-Hash
//  2. calculate Dataset-Revision-Hash for each modified Dataset and put into bucketDSR.
//     for each modified Dataset, update its stage.localGraphs[id].checkedOutDSRevisions to the new Dataset-Revision-Hash
//  3. create a new commit in bucketCommit, with all modified Dataset and commit info.
//  4. update bucketDataset with new commit Hash (and branch if any)
//  5. update Repository Log bucket
//
// Notes:
//
//	How to calculate Revision Hashes:
//	1. NamedGraph-Revision-Hash: sha256(sst file content)
//	2. Dataset-Revision-Hash: sha256(NamedGraph-Revision-Hash + Imported-NamedGraph-Revision-Hash)
//	3. Commit-Hash: sha256(Dataset-UUID + Dataset-Revision-Hash + Parent-Commit-Hash + Author + Timestamp + commit-message)
func (r *localFullRepository) commitNewVersion(ctx context.Context, stage *stage, message string, branchName string) (Hash, []uuid.UUID, error) {
	if err := r.enter(); err != nil {
		return emptyHash, nil, err
	}
	defer r.leave()

	if r.quota != nil {
		r.quota.writeMu.Lock()
		defer r.quota.writeMu.Unlock()
	}

	// handle bleve pre-commit constraints (the returned notifier is not used here;
	// LocalFullRepository performs branch-gated re-indexing via bleveIndexUpdater).
	if r.config.preCommitCondition != nil {
		_, err := r.config.preCommitCondition(stage)
		if err != nil {
			return HashNil(), nil, err
		}
	}
	var newCommitID Hash
	var commitTime time.Time

	var modifiedNGIDs []uuid.UUID
	deletedNGIDs := make(map[uuid.UUID]struct{})
	// find modified NGs
	for id, ng := range stage.localGraphs {
		if ng.flags.modified {
			modifiedNGIDs = append(modifiedNGIDs, id)
			GlobalLogger.Debug("Found modified Named Graph", zap.String("ngID", id.String()))
			// ng.flags.modified = false
		}

		if ng.flags.deleted {
			deletedNGIDs[id] = struct{}{}
		}
	}

	sort.Slice(modifiedNGIDs, func(i, j int) bool {
		return bytes.Compare(modifiedNGIDs[i][:], modifiedNGIDs[j][:]) < 0
	})

	if len(modifiedNGIDs) == 0 {
		return HashNil(), nil, ErrNothingToCommit
	}

	modifiedDSsMap := stage.modifiedDatasets()
	// fmt.Println("modifiedDSsMap:", modifiedDSsMap)

	// Capture pre-commit BBolt file size for quota delta calculation.
	var preBboltSize int64
	if fi, err := os.Stat(r.db.Path()); err == nil {
		preBboltSize = fi.Size()
	}

	err := r.db.Update(func(tx *bbolt.Tx) error {
		/* Start Filling Bucket NGR (NamedGraph Revision) */
		bucketNgr, err := tx.CreateBucketIfNotExists(keyNamedGraphRevisions)
		if err != nil {
			return nil
		}
		modifiedNGHashes := make([]Hash, len(modifiedNGIDs))
		if len(modifiedNGIDs) > 0 {
			err = stage.writeStageToSstFilesWithIDMatched(localFullFsOf(
				bucketNgr, modifiedNGIDs, modifiedNGHashes,
			), func(ngID uuid.UUID) bool {
				if _, found := deletedNGIDs[ngID]; found {
					return false
				}
				return true
			})
			if err != nil {
				GlobalLogger.Error("", zap.Error(err))
				return err
			}
		}

		// Quota check after the heavy BBolt write but before committing metadata.
		// tx.Size() reflects the database file size after the NGR writes.
		if r.sr != nil {
			bboltDelta := tx.Size() - preBboltSize
			if err := r.checkQuota(ctx, bboltDelta, 0); err != nil {
				return err
			}
		}

		// modifiedNGIDs and modifiedNGHashes in a map, for convenience
		modifiedNgMap := make(map[uuid.UUID]Hash, len(modifiedNGIDs))
		for i, value := range modifiedNGIDs {
			modifiedNgMap[value] = modifiedNGHashes[i]
			// after each commit, need update the checkedOutNGHash of the loadedGraph in the stage
			stage.localGraphs[value].checkedOutNGRevisions = []Hash{modifiedNGHashes[i]}
		}

		// set deleted NG Hash to emptyHash
		for key := range deletedNGIDs {
			modifiedNgMap[key] = emptyHash
			stage.localGraphs[key].checkedOutNGRevisions = []Hash{emptyHash}
		}

		/* Finish Filling Bucket NGR (NamedGraph Revision) */

		/* Start Filling Bucket DSR (DatasetRevision) */
		bucketDsr, err := tx.CreateBucketIfNotExists(keyDatasetRevisions)
		if err != nil {
			GlobalLogger.Error("", zap.Error(err))
			return err
		}

		type dsrStruct struct {
			generatedDSHash            Hash
			directImportsNGIdAndNGHash map[uuid.UUID]Hash
			allImportsNGIdAndNGHash    map[uuid.UUID]Hash
		}

		dsrMap := make(map[uuid.UUID]dsrStruct)

		// collect dsr information
		for i := range stage.localGraphs {
			tempDsrStruct := dsrStruct{}
			tempDsrStruct.allImportsNGIdAndNGHash = make(map[uuid.UUID]Hash)
			// generate a Hash as dsr
			dsHasher := sha256.New()
			ngHash := stage.localGraphs[i].lastCheckedOutNGRevision()
			// dsHasher.Write(i[:])
			dsHasher.Write(ngHash[:])
			tempAllImportsNGIdAndNGHash := make(map[uuid.UUID]Hash)

			// use all imported NGRevision hash to calculate the DatasetRevision hash, as long as it is imported,
			// no matter directly or indirectly imported.
			for _, importedNG := range stage.localGraphs[i].allImports() {
				// means this graph is new created, not loaded from another Repository
				if importedNG.(*namedGraph).lastCheckedOutNGRevision() == emptyHash {
					// use new generated Hash
					importedNG.(*namedGraph).checkedOutNGRevisions = []Hash{modifiedNgMap[importedNG.(*namedGraph).id]}
				}
				tempAllImportsNGIdAndNGHash[importedNG.(*namedGraph).id] = stage.localGraphs[importedNG.(*namedGraph).id].lastCheckedOutNGRevision()
			}
			tempDsrStruct.directImportsNGIdAndNGHash = tempAllImportsNGIdAndNGHash
			for _, hash := range tempAllImportsNGIdAndNGHash {
				// dsHasher.Write(id[:])
				dsHasher.Write(hash[:])
			}
			// dsHasher.Write([]byte(message))
			dsHash := dsHasher.Sum(nil)

			// if current NG's Dataset Hash is emptyHash, means new created
			if stage.localGraphs[i].lastCheckedOutDSRevision() == emptyHash {
				stage.localGraphs[i].checkedOutDSRevisions = []Hash{BytesToHash(dsHash)}
				tempDsrStruct.generatedDSHash = BytesToHash(dsHash)
			} else {
				// if it is modified, update
				if _, found := modifiedDSsMap[i]; found {
					tempDsrStruct.generatedDSHash = BytesToHash(dsHash)
					stage.localGraphs[i].checkedOutDSRevisions = []Hash{BytesToHash(dsHash)}
				} else {
					// not modified, use before
					tempDsrStruct.generatedDSHash = stage.localGraphs[i].lastCheckedOutDSRevision()
				}
			}

			dsrMap[i] = tempDsrStruct
		}

		// fill AllImportsIdAndHash by add directImportsIdAndHash recursively
		var fillAllImportsIdAndHash func(map[uuid.UUID]dsrStruct, dsrStruct, map[uuid.UUID]Hash)
		fillAllImportsIdAndHash = func(tempDsrMap map[uuid.UUID]dsrStruct, dsrS dsrStruct, directImportsIdAndHash map[uuid.UUID]Hash) {
			for importedID, importedHash := range directImportsIdAndHash {
				if _, found := dsrS.allImportsNGIdAndNGHash[importedID]; found {
					continue
				}
				dsrS.allImportsNGIdAndNGHash[importedID] = importedHash
				fillAllImportsIdAndHash(tempDsrMap, dsrS, dsrMap[importedID].directImportsNGIdAndNGHash)
			}
		}
		for _, dsrS := range dsrMap {
			fillAllImportsIdAndHash(dsrMap, dsrS, dsrS.directImportsNGIdAndNGHash)
		}

		// fill bucket DSR
		for id, dsrS := range dsrMap {
			// fmt.Printf("Dataset ID: %s, Generated DS Hash: %s\n", id.String(), dsrS.generatedDSHash.String())
			if _, found := modifiedDSsMap[id]; !found {
				continue
			}
			// according generated new dsHash to create sub-bucket in bucketDsr
			datasetRevision, err := bucketDsr.CreateBucketIfNotExists(dsrS.generatedDSHash[:])
			if err != nil {
				GlobalLogger.Error("", zap.Error(err))
				return err
			}

			// put current default NG-SHA
			defaultNGSha := modifiedNgMap[id]
			if defaultNGSha == zeroHash {
				defaultNGSha = stage.localGraphs[id].lastCheckedOutNGRevision()
			}

			// 	1. Default NG SHA hash (cardinality 1)
			// 		- key is "\x00"
			// 		- value is _NG-SHA_
			err = datasetRevision.Put([]byte{0x00}, defaultNGSha[:])
			if err != nil {
				GlobalLogger.Error("", zap.Error(err))
				return err
			}

			// 	2. Directly imported DSs (one by one) (cardinality 0 to *):
			// 		- key is byte '\x00' concatenated with imported _DS-UUID_
			// 		- value is _DS-SHA_
			for importsDsID := range dsrS.directImportsNGIdAndNGHash {
				hashBytes := make([]byte, len(dsrMap[importsDsID].generatedDSHash))
				hash := dsrMap[importsDsID].generatedDSHash
				copy(hashBytes, hash[:])
				// put imports id and hash
				err := datasetRevision.Put(iDToPrefixedKey(importsDsID, dsPrefix), hashBytes)
				if err != nil {
					GlobalLogger.Error("", zap.Error(err))
					return err
				}
				// fmt.Println("  Imported DS ID:", importsDsID.String(), " DS Hash:", hash.String())
			}

			// 	3. all (both direct and transitive) DS NGs (one by one) (cardinality 1 to *):
			// 		- key is byte '\x01' concatenated with _NG-UUID_
			// 		- value is _NG-SHA_
			for id, hash := range dsrS.allImportsNGIdAndNGHash {
				hashBytes := make([]byte, len(hash))
				copy(hashBytes, hash[:])
				err := datasetRevision.Put(iDToPrefixedKey(id, ngPrefix), hashBytes)
				if err != nil {
					GlobalLogger.Error("", zap.Error(err))
					return err
				}
			}
		}
		/* Finish Filling Bucket DSR (DatasetRevision) */

		/* Start Filling Bucket C (Commit) */
		bucketC, err := tx.CreateBucketIfNotExists(keyCommits)
		if err != nil {
			GlobalLogger.Error("", zap.Error(err))
			return err
		}

		commitTime = r.config.timeNow()
		unixCommitTime := commitTime.Unix()

		// start computing the SHA256 checksum
		commitHasher := sha256.New()
		buf := make([]byte, 10)
		for _, id := range modifiedNGIDs {
			// add each NG-ID
			commitHasher.Write(buf[:binary.PutUvarint(buf, uint64(len(id)))])
			commitHasher.Write(id[:])

			// add each NG Commit-Hash
			dsHash := dsrMap[id].generatedDSHash
			commitHasher.Write(buf[:binary.PutUvarint(buf, uint64(len(dsHash[:])))])
			commitHasher.Write(dsHash[:])
		}
		// add commit Author and email
		commitHasher.Write(buf[:binary.PutUvarint(buf, uint64(len(commitKeyAuthor)))])
		commitHasher.Write(commitKeyAuthor)

		bytesAuthorInfo := []byte(r.authInfo.Email)
		commitHasher.Write(buf[:binary.PutUvarint(buf, uint64(len(bytesAuthorInfo)+8))])
		commitHasher.Write(bytesAuthorInfo)

		// add commit time
		binary.BigEndian.PutUint64(buf, uint64(unixCommitTime))
		commitHasher.Write(buf[:8])

		// add commit message
		bytesMessage := stringAsImmutableBytes(message)
		commitHasher.Write(buf[:binary.PutUvarint(buf, uint64(len(commitKeyMessage)))])
		commitHasher.Write(commitKeyMessage)
		commitHasher.Write(buf[:binary.PutUvarint(buf, uint64(len(bytesMessage)))])
		commitHasher.Write(bytesMessage)

		commitHashBytes := commitHasher.Sum(nil)
		// finish computing Commit-Hash
		newCommitID = BytesToHash(commitHashBytes)

		bucketNewCommit, err := bucketC.CreateBucket(commitHashBytes)
		if err != nil {
			GlobalLogger.Error("", zap.Error(err))
			return err
		}
		var valueBuf bytes.Buffer
		for id := range modifiedDSsMap {
			valueBuf.Reset()
			// put current DS-SHA
			dsSha := dsrMap[id].generatedDSHash
			valueBuf.Write(dsSha[:])

			// put parent Commit-SHA
			bucketDs, err := tx.CreateBucketIfNotExists(keyDatasets)
			if err != nil {
				GlobalLogger.Error("", zap.Error(err))
				return err
			}
			subBucketDs := bucketDs.Bucket(id[:])
			// Get commits bucket (Bucket C) for validating commit hashes
			commitsBucket := tx.Bucket(keyCommits)
			if subBucketDs != nil {
				// When branchName is provided, look up the branch's current commit
				// Key: refBranchPrefix (0x00) + branch name, Value: commit hash
				if branchName != "" {
					branchKey := bytesToRefKey(stringAsImmutableBytes(branchName), refBranchPrefix)
					if branchCommitBytes := subBucketDs.Get(branchKey); branchCommitBytes != nil {
						// Validate that the branch commit exists in Bucket C
						if commitExistsInBucketC(commitsBucket, branchCommitBytes) {
							branchCommit := BytesToHash(branchCommitBytes)
							if len(stage.localGraphs[id].checkedOutCommits) > 0 {
								// checkedOutCommits is not empty, validate against branch's current commit
								branchCommitExistsInCheckedOut := false
								for _, val := range stage.localGraphs[id].checkedOutCommits {
									// Validate that the checkedOutCommit exists in Bucket C
									if commitExistsInBucketC(commitsBucket, val[:]) {
										if val == branchCommit {
											branchCommitExistsInCheckedOut = true
											break
										}
									}
								}

								for _, val := range stage.localGraphs[id].checkedOutCommits {
									// Validate that the checkedOutCommit exists in Bucket C
									if commitExistsInBucketC(commitsBucket, val[:]) {
										if branchCommitExistsInCheckedOut {
											valueBuf.Write(val[:])
										}
									}
								}
							} else {
								// checkedOutCommits is empty but branch has a commit
								// do not write a parent commit
							}
						} else {
							return fmt.Errorf("branch commit hash %s not found in commits bucket for dataset %s", BytesToHash(branchCommitBytes).String(), id.String())
						}
					}
				} else {
					// When no branchName, look for leaf commits
					// Key: refLeafPrefix (0x02) + commit hash, Value: commit hash
					for _, val := range stage.localGraphs[id].checkedOutCommits {
						// Validate that the checkedOutCommit exists in Bucket C
						if commitExistsInBucketC(commitsBucket, val[:]) {
							leafKey := bytesToRefKey(val[:], refLeafPrefix)
							if leafCommitBytes := subBucketDs.Get(leafKey); leafCommitBytes != nil {
								if val == BytesToHash(leafCommitBytes) {
									valueBuf.Write(val[:])
								}
							}
						} else {
							return fmt.Errorf("commit hash %s not found in commits bucket for dataset %s", BytesToHash(val[:]).String(), id.String())
						}
					}
				}
			}

			valBufBytes := valueBuf.Bytes()
			value := make([]byte, len(valBufBytes))
			copy(value, valBufBytes)
			err = bucketNewCommit.Put(iDToPrefixedKey(id, commitDsPrefix), value)
			if err != nil {
				GlobalLogger.Error("", zap.Error(err))
				return err
			}

		}
		valueBuf.Reset()
		valueBuf.Write([]byte(r.authInfo.Email))
		binary.BigEndian.PutUint64(buf, uint64(unixCommitTime))
		valueBuf.Write(buf[:8])
		valBufBytes := valueBuf.Bytes()
		value := make([]byte, len(valBufBytes))
		copy(value, valBufBytes)
		err = bucketNewCommit.Put(commitKeyAuthor, value)
		if err != nil {
			GlobalLogger.Error("", zap.Error(err))
			return err
		}
		err = bucketNewCommit.Put(commitKeyMessage, stringAsImmutableBytes(message))
		if err != nil {
			GlobalLogger.Error("", zap.Error(err))
			return err
		}
		// update NG checkoutCommit
		for _, value := range modifiedNGIDs {
			stage.localGraphs[value].checkedOutCommits = []Hash{newCommitID}
		}
		/* Finish Filling Bucket C (Commit) */

		/* Start Filling Bucket DS (Dataset) */
		for dsID := range modifiedDSsMap {
			bucketDs, err := tx.CreateBucketIfNotExists(keyDatasets)
			if err != nil {
				GlobalLogger.Error("", zap.Error(err))
				return err
			}
			newDsBucket, err := createDatasetBucketIfNotExists(bucketDs, dsID)
			if err != nil {
				GlobalLogger.Error("", zap.Error(err))
				return err
			}

			// Store IRI for Version 5 UUIDs
			if dsID.Version() == 5 {
				err = putDatasetIRI(newDsBucket, dsID, stage.localGraphs[dsID].baseIRI)
				if err != nil {
					GlobalLogger.Error("failed to store IRI for Version 5 UUID", zap.Error(err), zap.String("dsID", dsID.String()))
					return err
				}
			}

			// If branchName is empty string
			// key is refLeafPrefix concatenated with _Commit_Hash_
			// value is _Commit_Hash_
			if branchName == "" {
				// No branch specified, create as leaf commit
				if err := createLeafCommitRef(newDsBucket, commitHashBytes); err != nil {
					GlobalLogger.Error("", zap.Error(err))
					return err
				}
			} else {
				// Branch specified, check if there's an existing commit on this branch
				oldCommitBytes := getBranchCommitRef(newDsBucket, branchName)

				// Check if this is a deleted NG
				if _, deleted := deletedNGIDs[dsID]; deleted {
					// For deleted NG, convert branch to leaf (new commit becomes leaf)
					if err := convertBranchCommitToLeaf(newDsBucket, branchName, commitHashBytes); err != nil {
						GlobalLogger.Error("", zap.Error(err))
						return err
					}
				} else {
					// Normal commit (not deleted)
					// Check if the new created commit has a parent in Bucket C
					if oldCommitBytes != nil && !commitHasParentInBucketC(bucketC, dsID, commitHashBytes) {
						// New commit has no parent, convert old commit to leaf
						if err := convertBranchCommitToLeaf(newDsBucket, branchName, oldCommitBytes); err != nil {
							GlobalLogger.Error("", zap.Error(err))
							return err
						}
					}

					// Update branch to point to new commit
					if err := setBranchCommitRef(newDsBucket, branchName, commitHashBytes); err != nil {
						GlobalLogger.Error("", zap.Error(err))
						return err
					}
				}
			}
		}
		/* Finish Filling Bucket DS (Dataset) */

		/* Start Filling Bucket DSR commitHash*/
		for id, dsrS := range dsrMap {
			if _, found := modifiedDSsMap[id]; !found {
				continue
			}
			// according generated new dsHash to create sub-bucket in bucketDsr
			datasetRevision := bucketDsr.Bucket(dsrS.generatedDSHash[:])
			if datasetRevision == nil {
				GlobalLogger.Error("DatasetRevision bucket not found", zap.Error(errBucketNotExist))
				return errBucketNotExist
			}

			//  4. commitHash that created this Dataset Revision
			//      - key is "\x02"
			//      - value is commitHash
			err = datasetRevision.Put([]byte{0x02}, commitHashBytes)
			if err != nil {
				GlobalLogger.Error("", zap.Error(err))
				return err
			}

		}
		/* Finish Filling Bucket DSR commitHash*/

		// Start Filling Bucket RepositoryLog
		// Create or get top-level "log" bucket
		repoLogBucket, err := tx.CreateBucketIfNotExists([]byte("log"))
		if err != nil {
			return err
		}

		commitBase58 := newCommitID.String()

		fields := map[string]string{
			"type":      "commit",
			"commit_id": commitBase58,
			"branch":    branchName,
		}

		if err := writeRepositoryLogEntry(repoLogBucket, fields); err != nil {
			return err
		}

		// Finish Filling Bucket RepositoryLog

		return nil
	})
	if err != nil {
		GlobalLogger.Error("", zap.Error(err))
		return HashNil(), nil, err
	}

	commitInfo := commitInfo{}
	commitInfo.CommitHash = newCommitID.String()
	commitInfo.CommitAuthor = r.authInfo.Email
	commitInfo.CommitTime = commitTime.UTC().Format(time.RFC3339)
	commitInfo.CommitMessage = message

	modifiedDatasets := make([]uuid.UUID, 0)
	for key := range modifiedDSsMap {
		modifiedDatasets = append(modifiedDatasets, key)
	}

	if r.config.bleveIndexUpdater != nil {
		// LocalFullRepository does not use pcNotifier for post-commit index flushing;
		// pass nil so the updater only performs the branch-gated re-indexing.
		r.config.bleveIndexUpdater.updateAfterCommit(ctx, r, stage, nil, commitInfo, modifiedDatasets, branchName)
	}

	sort.SliceStable(modifiedDatasets, func(i, j int) bool {
		return bytes.Compare(modifiedDatasets[i][:], modifiedDatasets[j][:]) < 0
	})

	return newCommitID, modifiedDatasets, err
}

type datasetPushStream struct {
	bboltproto.DatasetService_PushDatasetsClient
}

func (s datasetPushStream) Send(obj *bboltproto.DatasetObject) error {
	return s.DatasetService_PushDatasetsClient.Send(&bboltproto.DatasetSuggestion{
		Sgg: &bboltproto.DatasetSuggestion_Obj{
			Obj: obj,
		},
	})
}

func datasetRevision(tx *bbolt.Tx, commits *bbolt.Bucket, dsID uuid.UUID, commitHash hashT) (*bbolt.Bucket, error) {
	commit := commits.Bucket(commitHash[:])
	if commit == nil {
		return nil, nil
	}
	dsKey := iDToPrefixedKey(dsID, commitDsPrefix)
	actualDSKey, dsHash := commit.Cursor().Seek(dsKey)
	if !bytes.Equal(actualDSKey, dsKey) {
		return nil, errDatasetRevisionMissing
	}
	datasetRevisions := tx.Bucket(keyDatasetRevisions)
	datasetRevision := datasetRevisions.Bucket(dsHash[:len(hashT{})])
	if datasetRevision == nil {
		return nil, errDatasetRevisionMissing
	}
	return datasetRevision, nil
}

func (r *localFullRepository) view(f func(rFS fs.FS) error) error {
	return r.db.View(func(tx *bbolt.Tx) error {
		datasets := tx.Bucket(keyDatasets)
		if datasets == nil {
			return nil
		}
		return f(datasetsFSOf(datasets))
	})
}

// calculateRepositoryVersionHash computes a SHA256 hash representing the content state of the repository.
// The hash is based on:
// - All dataset IRIs and their revision hashes for all branches (sorted by IRI, then by branch name)
// - All commit hashes (sorted)
// - All document hashes (sorted)
// This provides a quick way to check if a repository clone has the same content.
func (r *localFullRepository) calculateRepositoryVersionHash() (string, error) {
	var hashInput bytes.Buffer

	err := r.db.View(func(tx *bbolt.Tx) error {
		datasetsBucket := tx.Bucket(keyDatasets)
		commitsBucket := tx.Bucket(keyCommits)
		datasetRevisionsBucket := tx.Bucket(keyDatasetRevisions)
		namedGraphRevisionsBucket := tx.Bucket(keyNamedGraphRevisions)

		// 1. Collect all dataset IRIs and their revision hashes for all branches
		if datasetsBucket != nil {
			type datasetBranchRevision struct {
				iri    string
				branch string
				dsHash Hash
			}
			var datasetRevisions []datasetBranchRevision

			err := datasetsBucket.ForEach(func(k, v []byte) error {
				dsBucket := datasetsBucket.Bucket(k)
				if dsBucket == nil {
					return nil
				}

				dsID := uuid.UUID(*(*[len(uuid.UUID{})]byte)(k))

				// Get IRI once for this dataset (used for all branches)
				var dsIRI string
				iriBytes := dsBucket.Get([]byte{dsIRIPrefix})
				if iriBytes != nil {
					dsIRI = string(iriBytes)
				}

				// Iterate through all branches for this dataset
				err := dsBucket.ForEach(func(branchKey, commitHashBytes []byte) error {
					// Check if this is a branch (key starts with refBranchPrefix = 0x00)
					if len(branchKey) == 0 || branchKey[0] != byte(refBranchPrefix) {
						return nil // Not a branch, skip
					}

					if len(commitHashBytes) != 32 {
						return nil // Invalid commit hash, skip
					}

					branchName := string(branchKey[1:]) // Remove prefix byte
					commitHash := BytesToHash(commitHashBytes)

					// Get dataset revision hash from commit
					if commitsBucket != nil {
						commitBucket := commitsBucket.Bucket(commitHash[:])
						if commitBucket != nil {
							// Find dataset revision hash from commit
							dsKey := iDToPrefixedKey(dsID, commitDsPrefix)
							dsRevData := commitBucket.Get(dsKey)
							if len(dsRevData) >= 32 {
								dsRevHash := BytesToHash(dsRevData[:32])

								// Get IRI if not already retrieved
								if dsIRI == "" {
									if datasetRevisionsBucket != nil {
										dsrBucket := datasetRevisionsBucket.Bucket(dsRevHash[:])
										if dsrBucket != nil {
											// Get IRI from named graph revision
											ngKey := []byte{dsPrefix}
											ngHashBytes := dsrBucket.Get(ngKey)
											if len(ngHashBytes) == 32 && namedGraphRevisionsBucket != nil {
												ngBytes := namedGraphRevisionsBucket.Get(ngHashBytes)
												if ngBytes != nil {
													ngIRI, err := sstReadGraphURI(bufio.NewReader(bytes.NewBuffer(ngBytes)))
													if err == nil {
														dsIRI = strings.TrimSuffix(ngIRI, "#")
													}
												}
											}
											// Fallback to URN-UUID if still empty
											if dsIRI == "" {
												dsIRI = dsID.URN()
											}
										}
									}
								}

								if dsIRI != "" {
									datasetRevisions = append(datasetRevisions, datasetBranchRevision{
										iri:    dsIRI,
										branch: branchName,
										dsHash: dsRevHash,
									})
								}
							}
						}
					}

					return nil
				})
				return err
			})
			if err != nil {
				return err
			}

			// Sort by IRI, then by branch name for deterministic order
			sort.Slice(datasetRevisions, func(i, j int) bool {
				if datasetRevisions[i].iri != datasetRevisions[j].iri {
					return datasetRevisions[i].iri < datasetRevisions[j].iri
				}
				return datasetRevisions[i].branch < datasetRevisions[j].branch
			})

			// Write dataset revisions to hash input
			for _, dr := range datasetRevisions {
				hashInput.WriteString(dr.iri)
				hashInput.WriteString(dr.branch)
				hashInput.Write(dr.dsHash[:])
			}
		}

		// 2. Collect all commit hashes (sorted)
		if commitsBucket != nil {
			var commitHashes []Hash
			err := commitsBucket.ForEach(func(k, v []byte) error {
				commitBucket := commitsBucket.Bucket(k)
				if commitBucket != nil {
					commitHashes = append(commitHashes, BytesToHash(k))
				}
				return nil
			})
			if err != nil {
				return err
			}

			// Sort commit hashes for deterministic order
			sort.Slice(commitHashes, func(i, j int) bool {
				return bytes.Compare(commitHashes[i][:], commitHashes[j][:]) < 0
			})

			// Write commit hashes to hash input
			for _, ch := range commitHashes {
				hashInput.Write(ch[:])
			}
		}

		// 3. Collect all document hashes (sorted)
		docBucket := tx.Bucket([]byte(documentInfoBucket))
		if docBucket != nil {
			var docHashes []string
			err := docBucket.ForEach(func(k, v []byte) error {
				sub := docBucket.Bucket(k)
				if sub != nil {
					docHashes = append(docHashes, string(k))
				}
				return nil
			})
			if err != nil {
				return err
			}

			// Sort document hashes for deterministic order
			sort.Strings(docHashes)

			// Write document hashes to hash input
			for _, dh := range docHashes {
				hashInput.WriteString(dh)
			}
		}

		return nil
	})
	if err != nil {
		return "", fmt.Errorf("failed to calculate version hash: %w", err)
	}

	sum := sha256.Sum256(hashInput.Bytes())
	return Hash(sum).String(), nil
}

func (r *localFullRepository) Info(ctx context.Context, branchName string) (RepositoryInfo, error) {
	if err := r.enter(); err != nil {
		return RepositoryInfo{}, err
	}
	defer r.leave()

	// bbolt size
	BboltSize, err := getBboltSize(r.db)
	if err != nil {
		return RepositoryInfo{}, err
	}

	// Bleve size
	var bleveSize int64
	if r.bleveIndex != nil {
		bleveSize, err = r.getBleveSize()
		if err != nil {
			return RepositoryInfo{}, err
		}
	}

	NumberOfDatasets, NumberOfDatasetsInBranch, err := countDatasets(r.db, branchName)
	if err != nil {
		return RepositoryInfo{}, err
	}
	NumberOfDatasetRevisions, err := countDatasetRevisions(r.db)
	if err != nil {
		return RepositoryInfo{}, err
	}
	NumberOfNamedGraphRevisions, err := countNamedGraphRevisions(r.db)
	if err != nil {
		return RepositoryInfo{}, err
	}
	NumberOfCommits, err := countCommits(r.db)
	if err != nil {
		return RepositoryInfo{}, err
	}
	NumberOfRepositoryLogs, err := countRepositoryLogEntries(r.db)
	if err != nil {
		return RepositoryInfo{}, err
	}

	// Document DB size
	DocumentDBSize, err := calculateDocumentDBSize(r.config.repositoryDir)
	if err != nil {
		return RepositoryInfo{}, err
	}

	// Number of documents
	NumberOfDocuments, err := countDocuments(r.db)
	if err != nil {
		return RepositoryInfo{}, err
	}

	// Calculate version hash
	VersionHash, err := r.calculateRepositoryVersionHash()
	if err != nil {
		return RepositoryInfo{}, err
	}

	// Quota information
	var actualSize int64
	if fi, err := os.Stat(r.db.Path()); err == nil {
		if r.quota != nil {
			actualSize = fi.Size() + r.quota.documentSizeBytes.Load()
		}
	}
	var maxSize int64
	if r.sr != nil {
		name := filepath.Base(r.config.repositoryDir)
		if q, err := r.sr.GetQuota(ctx, name); err == nil {
			maxSize = q.MaxSizeBytes
		}
	}

	// return RepositoryStatistics
	return RepositoryInfo{
		URL: r.url.String(),
		// AccessRight was fixed at open/create time and never changes
		// during the lifetime of the repository instance.
		AccessRight:                 r.accessRight,
		MasterDBSize:                BboltSize,
		DerivedDBSize:               bleveSize,
		DocumentDBSize:              DocumentDBSize,
		ActualRepositorySize:        actualSize,
		MaxRepositorySize:           maxSize,
		NumberOfDatasets:            NumberOfDatasets,
		NumberOfDatasetsInBranch:    NumberOfDatasetsInBranch,
		NumberOfDatasetRevisions:    NumberOfDatasetRevisions,
		NumberOfNamedGraphRevisions: NumberOfNamedGraphRevisions,
		NumberOfCommits:             NumberOfCommits,
		NumberOfRepositoryLogs:      NumberOfRepositoryLogs,
		NumberOfDocuments:           NumberOfDocuments,
		IsRemote:                    false,
		SupportRevisionHistory:      true,
		BleveName:                   r.config.deriveInfo.DerivePackageName,
		BleveVersion:                r.config.deriveInfo.DerivePackageVersion,
		VersionHash:                 VersionHash,
	}, nil
}

// Log returns a list of log entries.
//   - `start`: starting logKey. If nil or 0, starts from latest.
//   - `end`: if negative, max number of entries.
//     if positive, end logKey (exclusive): logs strictly > end.
func (r *localFullRepository) Log(ctx context.Context, start, end *int) ([]RepositoryLogEntry, error) {
	if err := r.enter(); err != nil {
		return nil, err
	}
	defer r.leave()

	var results []RepositoryLogEntry

	err := r.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("log"))
		if bucket == nil {
			return nil
		}

		uint64ToKey := func(n uint64) []byte {
			b := make([]byte, 8)
			binary.BigEndian.PutUint64(b, n)
			return b
		}
		keyToUint64 := func(b []byte) uint64 {
			if len(b) != 8 {
				return 0
			}
			return binary.BigEndian.Uint64(b)
		}

		var startKey []byte
		var endKey []byte
		var maxCount int

		c := bucket.Cursor()

		// Default to last key if start is nil or 0
		if start == nil || *start == 0 {
			startKey, _ = c.Last()
		} else {
			startKey = uint64ToKey(uint64(*start))

			lastKey, _ := c.Last()
			if lastKey == nil || bytes.Compare(startKey, lastKey) > 0 {
				GlobalLogger.Debug(
					"log start key beyond max",
					zap.Int("start", *start),
					zap.Uint64("maxKey", keyToUint64(lastKey)),
				)
				return nil // return empty result
			}
		}

		// If end is negative, interpret as max count
		if end != nil && *end < 0 {
			maxCount = -*end
		} else if start != nil && end != nil {
			// Validate: in reverse range, start must >= end
			if *start < *end {
				GlobalLogger.Debug(
					"invalid log range in reverse mode",
					zap.Int("start", *start),
					zap.Int("end", *end),
				)
				return nil
			}
			endKey = uint64ToKey(uint64(*end))
		}

		// Traverse backward from startKey
		for k, _ := c.Seek(startKey); k != nil; k, _ = c.Prev() {
			if endKey != nil && bytes.Compare(k, endKey) <= 0 {
				break
			}
			if maxCount > 0 && len(results) >= maxCount {
				break
			}

			sub := bucket.Bucket(k)
			if sub == nil {
				continue
			}

			fields := make(map[string]string)
			_ = sub.ForEach(func(sk, sv []byte) error {
				fields[string(sk)] = string(sv)
				return nil
			})

			logKey := keyToUint64(k)

			results = append(results, RepositoryLogEntry{
				LogKey: logKey,
				Fields: fields,
			})
		}

		return nil
	})

	return results, err
}

func getBboltSize(db *bbolt.DB) (int64, error) {
	info, err := os.Stat(db.Path())
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

func (r *localFullRepository) getBlevePaths() (indexPath string, infoPath string) {
	blevePath := filepath.Join(r.config.repositoryDir, indexBleveDir)
	bleveInfoPath := filepath.Join(blevePath, bleveInfoName)
	return blevePath, bleveInfoPath
}

func (r *localFullRepository) getBleveSize() (int64, error) {
	blevePath, _ := r.getBlevePaths()

	if _, err := os.Stat(blevePath); os.IsNotExist(err) {
		return 0, fmt.Errorf("bleve index not initialized")
	}

	var size int64
	err := filepath.Walk(blevePath, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			// when counting size, if file not exist, just skip
			// this can happen if bleve is updating during the walk
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if info == nil || info.IsDir() {
			return nil
		}
		size += info.Size()
		return nil
	})
	if err != nil {
		return 0, err
	}
	return size, nil
}

func countDatasets(db *bbolt.DB, branchName string) (int64, int64, error) {
	var datasetCount int64
	var datasetCountInBranch int64
	err := db.View(func(tx *bbolt.Tx) error {
		bucketDS := tx.Bucket(keyDatasets)
		if bucketDS == nil {
			return nil
		}

		// count = bucket.Stats().KeyN
		datasetCount = 0
		datasetCountInBranch = 0
		err := bucketDS.ForEach(func(k, v []byte) error {
			datasetCount++

			specificDs := bucketDS.Bucket(k)
			if specificDs == nil {
				return fmt.Errorf("dataset sub-bucket missing for dataset %x", k)
			}
			isBranchFound := false
			specificDs.ForEach(func(sk, sv []byte) error {
				// check if branchName is "", then count all datasets with any branch
				if bytes.HasPrefix(sk, []byte{byte(refBranchPrefix)}) {
					if branchName == "" {
						isBranchFound = true
					} else {
						if bytes.HasSuffix(sk, []byte(branchName)) {
							isBranchFound = true
						}
					}
					return nil
				}

				return nil
			})
			if isBranchFound {
				datasetCountInBranch++
			}

			return nil
		})
		if err != nil {
			return fmt.Errorf("error iterating bucket: %w", err)
		}

		return nil
	})
	return datasetCount, datasetCountInBranch, err
}

func countDatasetRevisions(db *bbolt.DB) (int64, error) {
	var count int64
	err := db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(keyDatasetRevisions)
		if bucket == nil {
			return nil
		}

		// count = bucket.Stats().KeyN
		count = 0
		err := bucket.ForEach(func(k, v []byte) error {
			count++
			return nil
		})
		if err != nil {
			return fmt.Errorf("error iterating bucket: %w", err)
		}

		return nil
	})
	return count, err
}

func countNamedGraphRevisions(db *bbolt.DB) (int64, error) {
	var count int64
	err := db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(keyNamedGraphRevisions)
		if bucket == nil {
			return nil
		}

		// count = bucket.Stats().KeyN
		count = 0
		err := bucket.ForEach(func(k, v []byte) error {
			count++
			return nil
		})
		if err != nil {
			return fmt.Errorf("error iterating bucket: %w", err)
		}

		return nil
	})
	return count, err
}

func countCommits(db *bbolt.DB) (int64, error) {
	var count int64
	err := db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(keyCommits)
		if bucket == nil {
			return nil
		}

		// count = bucket.Stats().KeyN
		count = 0
		err := bucket.ForEach(func(k, v []byte) error {
			count++
			return nil
		})
		if err != nil {
			return fmt.Errorf("error iterating bucket: %w", err)
		}

		return nil
	})
	return count, err
}

func countRepositoryLogEntries(db *bbolt.DB) (int64, error) {
	var count int64
	err := db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("log"))
		if bucket == nil {
			return nil
		}

		// count = bucket.Stats().KeyN
		count = 0
		err := bucket.ForEach(func(k, v []byte) error {
			count++
			return nil
		})
		if err != nil {
			return fmt.Errorf("error iterating bucket: %w", err)
		}

		return nil
	})
	return count, err
}

func countDocuments(db *bbolt.DB) (int64, error) {
	var count int64
	err := db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(documentInfoBucket))
		if bucket == nil {
			return nil
		}

		count = 0
		err := bucket.ForEach(func(k, v []byte) error {
			// k is the document hash (as string), v is a sub-bucket
			sub := bucket.Bucket(k)
			if sub != nil {
				count++
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("error iterating document info bucket: %w", err)
		}

		return nil
	})
	return count, err
}

// initDocumentSize walks the document vault and initializes the atomic counter.
func (r *localFullRepository) initDocumentSize() error {
	if r.quota == nil {
		return nil
	}
	size, err := calculateDocumentDBSize(r.config.repositoryDir)
	if err != nil {
		return err
	}
	r.quota.documentSizeBytes.Store(size)
	return nil
}

// checkQuota verifies that writing bboltDelta and docDelta bytes does not exceed
// the per-repository quota or the SuperRepository total quota.
// Caller must hold r.quota.writeMu when r.quota is non-nil.
func (r *localFullRepository) checkQuota(ctx context.Context, bboltDelta, docDelta int64) error {
	if r.sr == nil || r.quota == nil {
		return nil
	}

	name := filepath.Base(r.config.repositoryDir)

	// Per-repository quota.
	repoQuota, err := r.sr.GetQuota(ctx, name)
	if err != nil {
		return err
	}
	if repoQuota.MaxSizeBytes > 0 {
		info, err := os.Stat(r.db.Path())
		if err != nil {
			return fmt.Errorf("stat bbolt db: %w", err)
		}
		docSize := r.quota.documentSizeBytes.Load()
		if info.Size()+docSize+bboltDelta+docDelta > repoQuota.MaxSizeBytes {
			return ErrQuotaExceeded
		}
	}

	// SuperRepository total quota (soft limit).
	totalQuota, err := r.sr.GetTotalQuota(ctx)
	if err != nil {
		return err
	}
	if totalQuota.MaxSizeBytes > 0 {
		if totalQuota.ActualSizeBytes+bboltDelta+docDelta > totalQuota.MaxSizeBytes {
			return ErrQuotaExceeded
		}
	}

	return nil
}

func calculateDocumentDBSize(repositoryDir string) (int64, error) {
	vaultDir := filepath.Join(repositoryDir, VaultDirName)

	// Check if vault directory exists
	if _, err := os.Stat(vaultDir); os.IsNotExist(err) {
		return 0, nil // No documents stored yet
	}

	var size int64
	err := filepath.Walk(vaultDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// when counting size, if file not exist, just skip
			// this can happen if documents are being updated during the walk
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if info == nil || info.IsDir() {
			return nil
		}
		size += info.Size()
		return nil
	})
	if err != nil {
		return 0, err
	}
	return size, nil
}

func nextLogKey(bucket *bbolt.Bucket) (uint64, *bbolt.Bucket, error) {
	for i := uint64(0); i < 1<<63; i++ {
		k := make([]byte, 8)
		binary.BigEndian.PutUint64(k, i)

		// If the key is already used, skip
		if bucket.Get(k) != nil || bucket.Bucket(k) != nil {
			continue
		}

		// Try to create a bucket at this key
		entryBucket, err := bucket.CreateBucket(k)
		if err != nil {
			return 0, nil, fmt.Errorf("failed to create log entry bucket at key %d: %w", i, err)
		}

		return i, entryBucket, nil
	}
	return 0, nil, fmt.Errorf("unable to find free log key")
}

func writeRepositoryLogEntry(bucket *bbolt.Bucket, fields map[string]string) error {
	_, entryBucket, err := nextLogKey(bucket)
	if err != nil {
		return err
	}
	for k, v := range fields {
		if err := entryBucket.Put([]byte(k), []byte(v)); err != nil {
			return err
		}
	}
	return nil
}

func (r *localFullRepository) DocumentSet(ctx context.Context, mimeType string, source *bufio.Reader) (Hash, error) {
	if err := r.enter(); err != nil {
		return emptyHash, err
	}
	defer r.leave()

	if r.quota != nil {
		r.quota.writeMu.Lock()
		defer r.quota.writeMu.Unlock()
	}

	// Step 0: Read content and compute hash
	var buf bytes.Buffer
	hasher := sha256.New()
	multiWriter := io.MultiWriter(&buf, hasher)

	written, err := io.Copy(multiWriter, source) // number of bytes read
	if err != nil {
		return emptyHash, fmt.Errorf("failed to read document for hashing: %w", err)
	}
	size := written

	var hash Hash
	copy(hash[:], hasher.Sum(nil))

	// Step 1: Prepare vault path
	vaultDir := filepath.Join(r.config.repositoryDir, VaultDirName)
	if err := os.MkdirAll(vaultDir, os.ModePerm); err != nil {
		return emptyHash, fmt.Errorf("failed to create vault directory: %w", err)
	}

	fullPath := vaultPath(vaultDir, hash)

	author := "default@semanticstep.net"
	if user := sstauth.SstUserInfoFromContext(ctx); user != nil && user.Email != "" {
		author = user.Email
	}

	// Step 2: Check if file exists
	if _, err := os.Stat(fullPath); err == nil {
		// Ensure document info exists
		if _, err := readDocumentInfo(r.db, hash); err != nil {
			_ = writeDocumentInfo(r.db, hash, &DocumentInfo{
				MIMEType:  mimeType,
				Author:    author,
				Timestamp: time.Now(),
			})
		}
		return hash, nil
	}

	// Step 2.5: Quota check before writing the vault file.
	if err := r.checkQuota(ctx, 0, size); err != nil {
		return emptyHash, err
	}

	// Step 3: Store file
	if err := storeStreamToVault(&buf, vaultDir, hash); err != nil {
		return emptyHash, fmt.Errorf("failed to store file: %w", err)
	}

	// Update document size counter to match physical state.
	if r.quota != nil {
		r.quota.documentSizeBytes.Add(size)
	}

	// Step 4: Store info
	docInfo := &DocumentInfo{
		MIMEType:  mimeType,
		Author:    author,
		Timestamp: time.Now(),
	}
	if err := writeDocumentInfo(r.db, hash, docInfo); err != nil {
		return emptyHash, fmt.Errorf("failed to write document info: %w", err)
	}

	// Step 5: Write upload log
	err = r.db.Update(func(tx *bbolt.Tx) error {
		repoLogBucket, err := tx.CreateBucketIfNotExists([]byte("log"))
		if err != nil {
			return err
		}

		fields := map[string]string{
			"type":       "upload_document",
			"message":    "upload document",
			"author":     docInfo.Author,
			"timestamp":  docInfo.Timestamp.UTC().Format(time.RFC3339),
			"mime_type":  docInfo.MIMEType,
			"hash":       hash.String(),
			"size_bytes": strconv.FormatInt(size, 10),
		}

		return writeRepositoryLogEntry(repoLogBucket, fields)
	})
	if err != nil {
		return emptyHash, fmt.Errorf("failed to write document upload log entry: %w", err)
	}

	return hash, nil
}

// Document retrieves the document and its metadata from the vault,
// writes the content to the provided writer (if not nil), and logs the download event when content is streamed.
func (r *localFullRepository) Document(ctx context.Context, hash Hash, target io.Writer) (*DocumentInfo, error) {
	if err := r.enter(); err != nil {
		return nil, err
	}
	defer r.leave()

	vaultDir := filepath.Join(r.config.repositoryDir, VaultDirName)
	fullPath := vaultPath(vaultDir, hash)

	// Step 1: Open the file
	file, err := os.Open(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("document not found: %w", err)
		}
		return nil, fmt.Errorf("failed to open document: %w", err)
	}
	defer file.Close()

	// Step 1.1: Hydrate size via Stat
	var size int64
	if fi, statErr := file.Stat(); statErr == nil {
		size = fi.Size()
	}

	// Step 1.2: Stream file content only if target is provided
	if target != nil {
		if _, err := io.Copy(target, file); err != nil {
			return nil, fmt.Errorf("failed to write document to target: %w", err)
		}
	}

	// Step 2: Read document info (DB does not include Size)
	docInfo, err := readDocumentInfo(r.db, hash)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve document info: %w", err)
	}
	docInfo.Size = size // Populate Size at runtime

	// Step 3: If content was streamed, write download log
	if target != nil {
		if err := r.db.Update(func(tx *bbolt.Tx) error {
			repoLogBucket, err := tx.CreateBucketIfNotExists([]byte("log"))
			if err != nil {
				return err
			}

			fields := map[string]string{
				"type":       "download_document",
				"message":    "download document",
				"author":     docInfo.Author,
				"timestamp":  time.Now().UTC().Format(time.RFC3339),
				"mime_type":  docInfo.MIMEType,
				"hash":       hash.String(),
				"size_bytes": strconv.FormatInt(docInfo.Size, 10),
			}
			return writeRepositoryLogEntry(repoLogBucket, fields)
		}); err != nil {
			return nil, fmt.Errorf("failed to write document download log entry: %w", err)
		}
	}

	return docInfo, nil
}

func (r *localFullRepository) Documents(ctx context.Context) ([]DocumentInfo, error) {
	if err := r.enter(); err != nil {
		return nil, err
	}
	defer r.leave()

	var results []DocumentInfo

	// Read metadata (without Size) inside a short read transaction.
	err := r.db.View(func(tx *bbolt.Tx) error {
		root := tx.Bucket([]byte(documentInfoBucket))
		if root == nil {
			// No document info bucket yet.
			return nil
		}

		return root.ForEach(func(k, _ []byte) error {
			sub := root.Bucket(k)
			if sub == nil {
				return nil
			}

			hash, err := StringToHash(string(k))
			if err != nil {
				return fmt.Errorf("invalid document hash key %q: %w", k, err)
			}

			mime := sub.Get([]byte("mime_type"))
			author := sub.Get([]byte("author"))
			timestampRaw := sub.Get([]byte("timestamp"))
			if mime == nil || author == nil || timestampRaw == nil {
				// Skip incomplete records silently.
				return nil
			}

			timestamp, err := time.Parse(time.RFC3339, string(timestampRaw))
			if err != nil {
				return fmt.Errorf("invalid timestamp format for %q: %w", k, err)
			}

			results = append(results, DocumentInfo{
				Hash:      hash,
				MIMEType:  string(mime),
				Author:    string(author),
				Timestamp: timestamp.UTC(),
				// Size will be hydrated after the transaction.
			})
			return nil
		})
	})
	if err != nil {
		return nil, err
	}

	// Hydrate Size from the filesystem outside the DB transaction.
	vaultDir := filepath.Join(r.config.repositoryDir, VaultDirName)
	for i := range results {
		fullPath := vaultPath(vaultDir, results[i].Hash)
		if fi, statErr := os.Stat(fullPath); statErr == nil {
			results[i].Size = fi.Size()
		} else {
			// If the file is missing/corrupted, leave Size = 0.
		}
	}

	return results, nil
}

func (r *localFullRepository) ForDocuments(ctx context.Context, c func(d DocumentInfo) error) error {
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

func (r *localFullRepository) DocumentDelete(ctx context.Context, hash Hash) error {
	if err := r.enter(); err != nil {
		return nil
	}
	defer r.leave()

	if r.quota != nil {
		r.quota.writeMu.Lock()
		defer r.quota.writeMu.Unlock()
	}

	vaultDir := filepath.Join(r.config.repositoryDir, VaultDirName)
	filePath := vaultPath(vaultDir, hash)

	// Step 0: Stat before deletion (for logging size)
	var deletedSize int64
	var sizeBytes string
	if fi, err := os.Stat(filePath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("document file not found for hash %s", hash.String())
		}
		return fmt.Errorf("failed to stat document file: %w", err)
	} else {
		deletedSize = fi.Size()
		sizeBytes = strconv.FormatInt(deletedSize, 10)
	}

	// Step 1: Delete the file
	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("failed to delete document file: %w", err)
	}

	// Update document size counter to match physical state.
	if r.quota != nil {
		r.quota.documentSizeBytes.Add(-deletedSize)
	}

	// Step 2: Delete metadata and write deletion log
	err := r.db.Update(func(tx *bbolt.Tx) error {
		author := "unknown"
		mimeType := "unknown"

		metaRoot := tx.Bucket([]byte(documentInfoBucket))
		if metaRoot != nil {
			sub := metaRoot.Bucket([]byte(hash.String()))
			if sub != nil {
				if a := sub.Get([]byte("author")); a != nil {
					author = string(a)
				}
				if m := sub.Get([]byte("mime_type")); m != nil {
					mimeType = string(m)
				}
				_ = metaRoot.DeleteBucket([]byte(hash.String()))
			}
		}

		// Write repository log entry
		repoLogBucket, err := tx.CreateBucketIfNotExists([]byte("log"))
		if err != nil {
			return err
		}

		fields := map[string]string{
			"type":       "delete_document",
			"message":    "delete document",
			"author":     author,
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
			"mime_type":  mimeType,
			"hash":       hash.String(),
			"size_bytes": sizeBytes,
		}
		return writeRepositoryLogEntry(repoLogBucket, fields)
	})
	if err != nil {
		return fmt.Errorf("failed to delete metadata or write log: %w", err)
	}

	return nil
}

func (r *localFullRepository) ExtractSstFile(ctx context.Context, namedGraphRevision Hash, w io.Writer) error {
	if err := r.enter(); err != nil {
		return err
	}
	defer r.leave()

	return r.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("ngr"))
		if bucket == nil {
			return fmt.Errorf("bucket 'ngr' not found")
		}

		value := bucket.Get(namedGraphRevision[:])
		if value == nil {
			return fmt.Errorf("namedGraphRevision %x not found in 'ngr' bucket", namedGraphRevision)
		}

		_, err := w.Write(value)
		if err != nil {
			return fmt.Errorf("failed to write SST data to writer: %w", err)
		}

		return nil
	})
}

// collectImportedDatasetsFromRevision recursively collects all imported datasets from a DatasetRevision.
// It traverses the import graph starting from the given DatasetRevision hash.
func collectImportedDatasetsFromRevision(
	dsrBucket *bbolt.Bucket,
	dsRevisionHash Hash,
	visitedRevisions map[Hash]struct{},
	result map[uuid.UUID]struct{},
) error {
	// Avoid cycles and repeated work
	if _, ok := visitedRevisions[dsRevisionHash]; ok {
		return nil
	}
	visitedRevisions[dsRevisionHash] = struct{}{}

	subDsrBucket := dsrBucket.Bucket(dsRevisionHash[:])
	if subDsrBucket == nil {
		// DatasetRevision not found, skip
		return nil
	}

	// Extract all direct imports from this DatasetRevision
	// Key format: \x00 + imported dataset UUID
	// Value: imported dataset revision hash
	return subDsrBucket.ForEach(func(k, v []byte) error {
		if len(k) > 1 && k[0] == '\x00' {
			importedDsID := uuid.UUID(k[1:])
			importedDsRevisionHash := BytesToHash(v)

			// Add to result set
			result[importedDsID] = struct{}{}

			// Recursively process imported dataset
			return collectImportedDatasetsFromRevision(dsrBucket, importedDsRevisionHash, visitedRevisions, result)
		}
		return nil
	})
}

// collectAllImportedDatasets collects all directly and indirectly imported datasets
// for the given initial dataset IDs from a localFullRepository.
// If branchName is not empty, only traverses the specified branch; otherwise traverses all branches.
func collectAllImportedDatasets(
	ctx context.Context,
	from *localFullRepository,
	initialDatasetIDs []uuid.UUID,
	branchName string,
) (map[uuid.UUID]struct{}, error) {
	result := make(map[uuid.UUID]struct{})
	visitedRevisions := make(map[Hash]struct{})

	// Add initial dataset IDs to result
	for _, id := range initialDatasetIDs {
		result[id] = struct{}{}
	}

	// If no initial datasets, return empty result
	if len(initialDatasetIDs) == 0 {
		return result, nil
	}

	err := from.db.View(func(tx *bbolt.Tx) error {
		datasetsBucket := tx.Bucket(keyDatasets)
		if datasetsBucket == nil {
			return nil // No datasets bucket, nothing to collect
		}

		dsrBucket := tx.Bucket(keyDatasetRevisions)
		if dsrBucket == nil {
			return nil // No dataset revisions bucket, nothing to collect
		}

		commitsBucket := tx.Bucket(keyCommits)
		if commitsBucket == nil {
			return nil // No commits bucket, nothing to collect
		}

		// For each initial dataset, find all its branches and collect imports from each branch
		for _, dsID := range initialDatasetIDs {
			dsBucket := datasetsBucket.Bucket(dsID[:])
			if dsBucket == nil {
				// Dataset not found, skip
				continue
			}

			// Find branches for this dataset
			// Key format: \x00 + branch name
			// Value: commit hash
			if !isAllBranches(branchName) {
				// Only process the specified branch
				branchKey := bytesToRefKey(stringAsImmutableBytes(branchName), refBranchPrefix)
				commitHashBytes := dsBucket.Get(branchKey)
				if commitHashBytes == nil {
					// Branch not found for this dataset, skip
					continue
				}
				commitHash := BytesToHash(commitHashBytes)

				// Get DatasetRevision hash from commit
				commitBucket := commitsBucket.Bucket(commitHash[:])
				if commitBucket == nil {
					continue // Commit not found, skip
				}

				// Find DatasetRevision hash for this dataset in the commit
				dsKey := iDToPrefixedKey(dsID, commitDsPrefix)
				actualDSKey, dsRevisionHashBytes := commitBucket.Cursor().Seek(dsKey)
				if !bytes.Equal(actualDSKey, dsKey) {
					continue // DatasetRevision not found in this commit, skip
				}

				dsRevisionHash := BytesToHash(dsRevisionHashBytes[:len(hashT{})])

				// Collect imports from this DatasetRevision
				if err := collectImportedDatasetsFromRevision(dsrBucket, dsRevisionHash, visitedRevisions, result); err != nil {
					return err
				}
			} else {
				// Process all branches
				dsBucket.ForEach(func(k, v []byte) error {
					if len(k) > 1 && k[0] == '\x00' {
						// This is a branch
						commitHash := BytesToHash(v)

						// Get DatasetRevision hash from commit
						commitBucket := commitsBucket.Bucket(commitHash[:])
						if commitBucket == nil {
							return nil // Commit not found, skip
						}

						// Find DatasetRevision hash for this dataset in the commit
						dsKey := iDToPrefixedKey(dsID, commitDsPrefix)
						actualDSKey, dsRevisionHashBytes := commitBucket.Cursor().Seek(dsKey)
						if !bytes.Equal(actualDSKey, dsKey) {
							return nil // DatasetRevision not found in this commit, skip
						}

						dsRevisionHash := BytesToHash(dsRevisionHashBytes[:len(hashT{})])

						// Collect imports from this DatasetRevision
						if err := collectImportedDatasetsFromRevision(dsrBucket, dsRevisionHash, visitedRevisions, result); err != nil {
							return err
						}
					}
					return nil
				})
			}
		}

		return nil
	})

	return result, err
}

// SyncFrom synchronizes data from the source repository to this repository.
// It copies NamedGraphRevisions, DatasetRevisions, Datasets, Commits, and document_info.
// Options can be used to specify which datasets and branches to sync.
func (r *localFullRepository) SyncFrom(ctx context.Context, from Repository, options ...SyncOption) error {
	if err := r.enter(); err != nil {
		return err
	}
	defer r.leave()

	// Parse sync options
	opts := defaultSyncOptions()
	for _, option := range options {
		option(&opts)
	}

	// Extract DatasetIDs and BranchName from options
	datasetIDs := opts.DatasetIDs
	branchName := opts.BranchName

	// Check if source is a LocalFullRepository
	fromLocal, ok := from.(*localFullRepository)
	if ok {
		// Collect all imported datasets if datasetIDs is provided
		var datasetIDSet map[uuid.UUID]struct{}
		if len(datasetIDs) > 0 {
			var err error
			datasetIDSet, err = collectAllImportedDatasets(ctx, fromLocal, datasetIDs, branchName)
			if err != nil {
				return fmt.Errorf("failed to collect imported datasets: %w", err)
			}
		}

		existingDSRs, err := snapshotExistingDSRs(r.db)
		if err != nil {
			return fmt.Errorf("failed to snapshot existing dataset revisions: %w", err)
		}

		// Local to local sync
		// Step 1: Sync NamedGraphRevisions
		if err := r.syncNamedGraphRevisions(ctx, fromLocal, datasetIDSet, branchName); err != nil {
			return fmt.Errorf("failed to sync NamedGraphRevisions: %w", err)
		}

		// Step 2: Sync DatasetRevisions
		if err := r.syncDatasetRevisions(ctx, fromLocal, datasetIDSet, branchName); err != nil {
			return fmt.Errorf("failed to sync DatasetRevisions: %w", err)
		}

		// Step 3: Sync Datasets (with BleveIndex updates for master branch)
		if err := r.syncDatasets(ctx, fromLocal, datasetIDSet, branchName); err != nil {
			return fmt.Errorf("failed to sync Datasets: %w", err)
		}

		// Step 4: Sync Commits
		if err := r.syncCommits(ctx, fromLocal, datasetIDSet, branchName); err != nil {
			return fmt.Errorf("failed to sync Commits: %w", err)
		}

		// Step 5: Sync document_info
		if err := r.syncDocumentInfo(ctx, fromLocal, datasetIDSet); err != nil {
			return fmt.Errorf("failed to sync document_info: %w", err)
		}

		if err := writeSyncFromLogAfterSync(ctx, r, fromLocal.URL(), existingDSRs, branchName); err != nil {
			return fmt.Errorf("failed to write sync_from log entry: %w", err)
		}

		return nil
	}

	// Check if source is a RemoteRepository
	fromRemote, ok := from.(*remoteRepository)
	if ok {
		return r.syncFromRemote(ctx, fromRemote, options...)
	}

	// Unsupported repository type
	return fmt.Errorf("syncing from %T is not yet implemented", from)
}

// isNamedGraphRevisionForDatasets checks if a NamedGraphRevision belongs to any of the specified datasets.
// It does this by checking if the NamedGraphRevision hash is referenced in any DatasetRevision of the specified datasets.
// If branchName is specified, only checks commits from that branch.
func isNamedGraphRevisionForDatasets(
	ngRevisionHash Hash,
	datasetIDSet map[uuid.UUID]struct{},
	branchName string,
	dsrBucket *bbolt.Bucket,
	datasetsBucket *bbolt.Bucket,
	commitsBucket *bbolt.Bucket,
) bool {
	if datasetIDSet == nil && isAllBranches(branchName) {
		return true // No filter, include all
	}

	found := false

	// Determine which datasets to check
	datasetsToCheck := datasetIDSet
	if datasetsToCheck == nil {
		// If no dataset filter, check all datasets
		datasetsToCheck = make(map[uuid.UUID]struct{})
		datasetsBucket.ForEach(func(k, v []byte) error {
			if datasetsBucket.Bucket(k) != nil {
				datasetsToCheck[uuid.UUID(k)] = struct{}{}
			}
			return nil
		})
	}

	// Check all datasets in the set
	for dsID := range datasetsToCheck {
		if found {
			break
		}
		dsBucket := datasetsBucket.Bucket(dsID[:])
		if dsBucket == nil {
			continue
		}

		// Check branches
		if !isAllBranches(branchName) {
			// Only check the specified branch
			branchKey := bytesToRefKey(stringAsImmutableBytes(branchName), refBranchPrefix)
			commitHashBytes := dsBucket.Get(branchKey)
			if commitHashBytes == nil {
				continue // Branch not found for this dataset, skip
			}
			commitHash := BytesToHash(commitHashBytes)
			commitBucket := commitsBucket.Bucket(commitHash[:])
			if commitBucket == nil {
				continue
			}

			// Get DatasetRevision hash from commit
			dsKey := iDToPrefixedKey(dsID, commitDsPrefix)
			actualDSKey, dsRevisionHashBytes := commitBucket.Cursor().Seek(dsKey)
			if !bytes.Equal(actualDSKey, dsKey) {
				continue
			}

			dsRevisionHash := BytesToHash(dsRevisionHashBytes[:len(hashT{})])
			dsrSubBucket := dsrBucket.Bucket(dsRevisionHash[:])
			if dsrSubBucket == nil {
				continue
			}

			// Check if this NamedGraphRevision is referenced in the DatasetRevision
			dsrSubBucket.ForEach(func(dsrK, dsrV []byte) error {
				if (len(dsrK) == 1 && dsrK[0] == '\x00') || (len(dsrK) > 1 && dsrK[0] == '\x01') {
					if bytes.Equal(dsrV, ngRevisionHash[:]) {
						found = true
						return nil
					}
				}
				return nil
			})
		} else {
			// Check all branches of this dataset
			dsBucket.ForEach(func(k, v []byte) error {
				if found {
					return nil // Already found, skip
				}
				if len(k) > 1 && k[0] == '\x00' {
					// This is a branch, v is commit hash
					commitHash := BytesToHash(v)
					commitBucket := commitsBucket.Bucket(commitHash[:])
					if commitBucket == nil {
						return nil
					}

					// Get DatasetRevision hash from commit
					dsKey := iDToPrefixedKey(dsID, commitDsPrefix)
					actualDSKey, dsRevisionHashBytes := commitBucket.Cursor().Seek(dsKey)
					if !bytes.Equal(actualDSKey, dsKey) {
						return nil
					}

					dsRevisionHash := BytesToHash(dsRevisionHashBytes[:len(hashT{})])
					dsrSubBucket := dsrBucket.Bucket(dsRevisionHash[:])
					if dsrSubBucket == nil {
						return nil
					}

					// Check if this NamedGraphRevision is referenced in the DatasetRevision
					// Default NamedGraph: key is \x00, value is NGR hash
					// Imported NamedGraph: key is \x01 + NG UUID, value is NGR hash
					dsrSubBucket.ForEach(func(dsrK, dsrV []byte) error {
						if (len(dsrK) == 1 && dsrK[0] == '\x00') || (len(dsrK) > 1 && dsrK[0] == '\x01') {
							if bytes.Equal(dsrV, ngRevisionHash[:]) {
								found = true
								return nil
							}
						}
						return nil
					})
				}
				return nil
			})
		}
	}

	return found
}

// syncNamedGraphRevisions copies NamedGraphRevisions from source to target if not present.
// If datasetIDSet is not nil, only syncs NamedGraphRevisions that belong to the specified datasets.
// If branchName is not empty, only syncs NamedGraphRevisions from the specified branch.
func (r *localFullRepository) syncNamedGraphRevisions(ctx context.Context, from *localFullRepository, datasetIDSet map[uuid.UUID]struct{}, branchName string) error {
	return from.db.View(func(fromTx *bbolt.Tx) error {
		fromBucket := fromTx.Bucket(keyNamedGraphRevisions)
		if fromBucket == nil {
			return nil // Source has no NamedGraphRevisions bucket
		}

		// Get buckets needed for filtering
		var dsrBucket, datasetsBucket, commitsBucket *bbolt.Bucket
		if datasetIDSet != nil || !isAllBranches(branchName) {
			dsrBucket = fromTx.Bucket(keyDatasetRevisions)
			datasetsBucket = fromTx.Bucket(keyDatasets)
			commitsBucket = fromTx.Bucket(keyCommits)
		}

		return r.db.Update(func(toTx *bbolt.Tx) error {
			toBucket, err := toTx.CreateBucketIfNotExists(keyNamedGraphRevisions)
			if err != nil {
				return err
			}

			return fromBucket.ForEach(func(k, v []byte) error {
				// Check if already exists in target
				if toBucket.Get(k) != nil {
					return nil // Already exists, skip
				}

				// Filter by datasetIDSet and/or branchName if provided
				if datasetIDSet != nil || !isAllBranches(branchName) {
					ngRevisionHash := BytesToHash(k)
					if !isNamedGraphRevisionForDatasets(ngRevisionHash, datasetIDSet, branchName, dsrBucket, datasetsBucket, commitsBucket) {
						return nil // Not for specified datasets/branch, skip
					}
				}

				// Copy the NamedGraphRevision
				return toBucket.Put(k, v)
			})
		})
	})
}

// isDatasetRevisionForDatasets checks if a DatasetRevision belongs to any of the specified datasets.
// It does this by checking if the DatasetRevision hash is used in any commit of the specified datasets.
// If branchName is specified, only checks commits from that branch.
func isDatasetRevisionForDatasets(
	dsRevisionHash Hash,
	datasetIDSet map[uuid.UUID]struct{},
	branchName string,
	datasetsBucket *bbolt.Bucket,
	commitsBucket *bbolt.Bucket,
) bool {
	if datasetIDSet == nil && isAllBranches(branchName) {
		return true // No filter, include all
	}

	found := false

	// Determine which datasets to check
	datasetsToCheck := datasetIDSet
	if datasetsToCheck == nil {
		// If no dataset filter, check all datasets
		datasetsToCheck = make(map[uuid.UUID]struct{})
		datasetsBucket.ForEach(func(k, v []byte) error {
			if datasetsBucket.Bucket(k) != nil {
				datasetsToCheck[uuid.UUID(k)] = struct{}{}
			}
			return nil
		})
	}

	// Check all datasets in the set
	for dsID := range datasetsToCheck {
		if found {
			break
		}
		dsBucket := datasetsBucket.Bucket(dsID[:])
		if dsBucket == nil {
			continue
		}

		// Check branches
		if !isAllBranches(branchName) {
			// Only check the specified branch
			branchKey := bytesToRefKey(stringAsImmutableBytes(branchName), refBranchPrefix)
			commitHashBytes := dsBucket.Get(branchKey)
			if commitHashBytes == nil {
				continue // Branch not found for this dataset, skip
			}
			commitHash := BytesToHash(commitHashBytes)
			commitBucket := commitsBucket.Bucket(commitHash[:])
			if commitBucket == nil {
				continue
			}

			// Get DatasetRevision hash from commit
			dsKey := iDToPrefixedKey(dsID, commitDsPrefix)
			actualDSKey, dsRevisionHashBytes := commitBucket.Cursor().Seek(dsKey)
			if !bytes.Equal(actualDSKey, dsKey) {
				continue
			}

			// Check if this matches the DatasetRevision we're looking for
			if bytes.Equal(dsRevisionHashBytes[:len(hashT{})], dsRevisionHash[:]) {
				found = true
				continue
			}

			// Also check imported datasets - the DatasetRevision might be for an imported dataset
			// We need to check if this DatasetRevision is referenced in the import chain
			// This is a simplified check - a full implementation would need to traverse the import graph
		} else {
			// Check all branches of this dataset
			dsBucket.ForEach(func(k, v []byte) error {
				if found {
					return nil // Already found, skip
				}
				if len(k) > 1 && k[0] == '\x00' {
					// This is a branch, v is commit hash
					commitHash := BytesToHash(v)
					commitBucket := commitsBucket.Bucket(commitHash[:])
					if commitBucket == nil {
						return nil
					}

					// Get DatasetRevision hash from commit
					dsKey := iDToPrefixedKey(dsID, commitDsPrefix)
					actualDSKey, dsRevisionHashBytes := commitBucket.Cursor().Seek(dsKey)
					if !bytes.Equal(actualDSKey, dsKey) {
						return nil
					}

					// Check if this matches the DatasetRevision we're looking for
					if bytes.Equal(dsRevisionHashBytes[:len(hashT{})], dsRevisionHash[:]) {
						found = true
						return nil
					}

					// Also check imported datasets - the DatasetRevision might be for an imported dataset
					// We need to check if this DatasetRevision is referenced in the import chain
					// This is a simplified check - a full implementation would need to traverse the import graph
				}
				return nil
			})
		}
	}

	return found
}

// syncDatasetRevisions copies DatasetRevisions from source to target if not present.
// If datasetIDSet is not nil, only syncs DatasetRevisions that belong to the specified datasets.
// If branchName is not empty, only syncs DatasetRevisions from the specified branch.
func (r *localFullRepository) syncDatasetRevisions(ctx context.Context, from *localFullRepository, datasetIDSet map[uuid.UUID]struct{}, branchName string) error {
	return from.db.View(func(fromTx *bbolt.Tx) error {
		fromBucket := fromTx.Bucket(keyDatasetRevisions)
		if fromBucket == nil {
			return nil // Source has no DatasetRevisions bucket
		}

		// Get buckets needed for filtering
		var datasetsBucket, commitsBucket *bbolt.Bucket
		if datasetIDSet != nil || !isAllBranches(branchName) {
			datasetsBucket = fromTx.Bucket(keyDatasets)
			commitsBucket = fromTx.Bucket(keyCommits)
		}

		return r.db.Update(func(toTx *bbolt.Tx) error {
			toBucket, err := toTx.CreateBucketIfNotExists(keyDatasetRevisions)
			if err != nil {
				return err
			}

			return fromBucket.ForEach(func(k, v []byte) error {
				// k is the DatasetRevision hash, v is a sub-bucket
				fromSubBucket := fromBucket.Bucket(k)
				if fromSubBucket == nil {
					return nil // Not a bucket, skip
				}

				// Filter by datasetIDSet and/or branchName if provided
				if datasetIDSet != nil || !isAllBranches(branchName) {
					dsRevisionHash := BytesToHash(k)
					if !isDatasetRevisionForDatasets(dsRevisionHash, datasetIDSet, branchName, datasetsBucket, commitsBucket) {
						return nil // Not for specified datasets/branch, skip
					}
				}

				// Check if already exists in target
				if toBucket.Bucket(k) != nil {
					return nil // Already exists, skip
				}

				// Create sub-bucket in target
				toSubBucket, err := toBucket.CreateBucket(k)
				if err != nil {
					return err
				}

				// Copy all entries from sub-bucket
				return fromSubBucket.ForEach(func(sk, sv []byte) error {
					return toSubBucket.Put(sk, sv)
				})
			})
		})
	})
}

// syncDatasets copies Datasets from source to target if not present
// If they exist and have identical content, skip
// Otherwise, if they exist and have different content, perform detailed analysis
// Updates BleveIndex as needed (if master branch)
// If datasetIDSet is not nil, only syncs datasets in the set.
// If branchName is not empty, only syncs datasets from the specified branch.
func (r *localFullRepository) syncDatasets(ctx context.Context, from *localFullRepository, datasetIDSet map[uuid.UUID]struct{}, branchName string) error {
	return from.db.View(func(fromTx *bbolt.Tx) error {
		fromBucket := fromTx.Bucket(keyDatasets)
		if fromBucket == nil {
			return nil // Source has no Datasets bucket
		}

		return r.db.Update(func(toTx *bbolt.Tx) error {
			toBucket, err := toTx.CreateBucketIfNotExists(keyDatasets)
			if err != nil {
				return err
			}

			return fromBucket.ForEach(func(k, v []byte) error {
				// k is the Dataset UUID, v is a sub-bucket
				fromSubBucket := fromBucket.Bucket(k)
				if fromSubBucket == nil {
					return nil // Not a bucket, skip
				}

				// Filter by datasetIDSet if provided
				if datasetIDSet != nil {
					dsID := uuid.UUID(k)
					if _, ok := datasetIDSet[dsID]; !ok {
						return nil // Dataset not in the set, skip
					}
				}

				// If branchName is specified, check if this dataset has the branch
				if !isAllBranches(branchName) {
					branchKey := bytesToRefKey(stringAsImmutableBytes(branchName), refBranchPrefix)
					if fromSubBucket.Get(branchKey) == nil {
						// Branch not found for this dataset, skip
						return nil
					}
				}

				toSubBucket := toBucket.Bucket(k)
				if toSubBucket == nil {
					// Dataset doesn't exist in target, create it
					toSubBucket, err = toBucket.CreateBucket(k)
					if err != nil {
						return err
					}

					if !isAllBranches(branchName) {
						// Copy only the specified branch entry
						branchKey := bytesToRefKey(stringAsImmutableBytes(branchName), refBranchPrefix)
						branchValue := fromSubBucket.Get(branchKey)
						if branchValue != nil {
							if err := toSubBucket.Put(branchKey, branchValue); err != nil {
								return err
							}
						}
					} else {
						// Copy all entries from source
						if err := fromSubBucket.ForEach(func(sk, sv []byte) error {
							return toSubBucket.Put(sk, sv)
						}); err != nil {
							return err
						}
					}

					// Update BleveIndex if master branch exists
					if r.config.bleveIndexUpdater != nil {
						r.config.bleveIndexUpdater.updateAfterSync(ctx, r, uuid.UUID(k), DefaultBranch)
					}

					return nil
				}

				// Dataset exists, handle branch-specific sync
				if !isAllBranches(branchName) {
					// Sync only the specified branch
					branchKey := bytesToRefKey(stringAsImmutableBytes(branchName), refBranchPrefix)
					branchValue := fromSubBucket.Get(branchKey)
					if branchValue != nil {
						// Update or add the branch entry
						if err := toSubBucket.Put(branchKey, branchValue); err != nil {
							return err
						}
					}
					return nil
				}

				// All branches: check if content is identical
				if r.datasetsAreIdentical(fromSubBucket, toSubBucket) {
					return nil // Identical, skip
				}

				// Different content - perform detailed analysis
				// TO BE SPECIFIED: A more detailed analysis of all revisions and all Named Branches
				// and their parents have to happen here.
				// For now, we skip this dataset and log a warning
				GlobalLogger.Warn("Dataset exists with different content - detailed analysis TO BE SPECIFIED",
					zap.String("dataset", uuid.UUID(k).String()))
				// TODO: Implement detailed analysis of revisions and branches
				// TODO: Update BleveIndex as needed (if master branch) after analysis

				return nil
			})
		})
	})
}

// datasetsAreIdentical checks if two dataset buckets have identical content.
func (r *localFullRepository) datasetsAreIdentical(bucket1, bucket2 *bbolt.Bucket) bool {
	// Compare all key-value pairs
	seen := make(map[string][]byte)
	err := bucket1.ForEach(func(k, v []byte) error {
		seen[string(k)] = v
		return nil
	})
	if err != nil {
		return false
	}

	identical := true
	err = bucket2.ForEach(func(k, v []byte) error {
		if val, ok := seen[string(k)]; !ok || !bytes.Equal(val, v) {
			identical = false
		}
		delete(seen, string(k))
		return nil
	})
	if err != nil {
		return false
	}

	// Check if all keys from bucket1 were found in bucket2
	return identical && len(seen) == 0
}

// syncFromRemote synchronizes data from a RemoteRepository to this LocalFullRepository via gRPC.
func (r *localFullRepository) syncFromRemote(ctx context.Context, fromRemote *remoteRepository, options ...SyncOption) error {
	// Parse sync options
	syncOpts := defaultSyncOptions()
	for _, option := range options {
		option(&syncOpts)
	}

	// Extract DatasetIDs and BranchName from options
	datasetIDs := syncOpts.DatasetIDs
	branchName := syncOpts.BranchName

	// Setup authentication
	var grpcOpts []grpc.CallOption
	authProvider := sstauth.AuthProviderFromContext(ctx)
	if p, ok := authProvider.(sstauth.Provider); ok {
		token, err := p.Oauth2Token()
		if err != nil {
			return fmt.Errorf("failed to get oauth token: %w", err)
		}
		if token != nil && token.AccessToken != "" {
			grpcOpts = append(grpcOpts, grpc.PerRPCCredentials(oauth.NewOauthAccess(token)))
		}
	}

	// Prepare metadata with dataset IDs and branch name
	metadata := &bboltproto.SyncToMetadata{
		BranchName: branchName,
	}
	if len(datasetIDs) > 0 {
		metadata.DatasetIds = make([][]byte, len(datasetIDs))
		for i, dsID := range datasetIDs {
			metadata.DatasetIds[i] = dsID[:]
		}
	}

	// Create gRPC stream to receive bucket data from server
	GlobalLogger.Debug("gRPC dsClient call SyncTo")
	stream, err := fromRemote.dsClient.SyncTo(ctx, &bboltproto.SyncToRequest{
		Metadata: metadata,
	}, grpcOpts...)
	if err != nil {
		return fmt.Errorf("failed to initiate SyncTo stream: %w", err)
	}

	// Track document_info hashes that need file sync
	documentHashesToSync := make(map[string]bool)

	existingDSRs, err := snapshotExistingDSRs(r.db)
	if err != nil {
		return fmt.Errorf("failed to snapshot existing dataset revisions: %w", err)
	}

	// Process streamed bucket data. Filtering is done server-side in SyncTo;
	// do not re-filter on the client with an incomplete dataset set.
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to receive sync data: %w", err)
		}

		switch data := resp.Data.(type) {
		case *bboltproto.SyncToResponse_BucketData:
			bd := data.BucketData
			if err := r.processBucketDataFromRemote(ctx, bd, documentHashesToSync, nil); err != nil {
				return fmt.Errorf("failed to process bucket data: %w", err)
			}

		case *bboltproto.SyncToResponse_Complete:
			complete := data.Complete
			GlobalLogger.Debug(
				"SyncTo completed",
				zap.Int32("namedGraphRevisions", complete.NamedGraphRevisionsSynced),
				zap.Int32("datasetRevisions", complete.DatasetRevisionsSynced),
				zap.Int32("datasets", complete.DatasetsSynced),
				zap.Int32("commits", complete.CommitsSynced),
				zap.Int32("documentInfo", complete.DocumentInfoSynced),
			)

			// Sync actual files for all document_info that were synced
			if err := r.syncDocumentFilesFromRemote(ctx, fromRemote, documentHashesToSync); err != nil {
				GlobalLogger.Warn("failed to sync some document files from remote", zap.Error(err))
				// Don't fail the entire sync if some files fail
			}

			if err := writeSyncFromLogAfterSync(ctx, r, fromRemote.URL(), existingDSRs, branchName); err != nil {
				return fmt.Errorf("failed to write sync_from log entry: %w", err)
			}

			return nil
		}
	}

	// Sync actual files for all document_info that were synced (in case Complete message was not received)
	if err := r.syncDocumentFilesFromRemote(ctx, fromRemote, documentHashesToSync); err != nil {
		GlobalLogger.Warn("failed to sync some document files from remote", zap.Error(err))
		// Don't fail the entire sync if some files fail
	}

	if err := writeSyncFromLogAfterSync(ctx, r, fromRemote.URL(), existingDSRs, branchName); err != nil {
		return fmt.Errorf("failed to write sync_from log entry: %w", err)
	}

	return nil
}

// syncDocumentFilesFromRemote syncs actual document files from remote repository.
func (r *localFullRepository) syncDocumentFilesFromRemote(ctx context.Context, fromRemote *remoteRepository, documentHashes map[string]bool) error {
	vaultDir := filepath.Join(r.config.repositoryDir, VaultDirName)
	if err := os.MkdirAll(vaultDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create vault directory: %w", err)
	}

	for hashStr := range documentHashes {
		hash, err := StringToHash(hashStr)
		if err != nil {
			GlobalLogger.Warn("invalid document hash during sync", zap.String("hash", hashStr), zap.Error(err))
			continue
		}

		// Check if file already exists in target
		fullPath := vaultPath(vaultDir, hash)
		if _, err := os.Stat(fullPath); err == nil {
			// File already exists, skip
			continue
		}

		// Read document from remote repository
		var buf bytes.Buffer
		docInfo, err := fromRemote.Document(ctx, hash, &buf)
		if err != nil {
			GlobalLogger.Warn("failed to read document from remote repository during sync",
				zap.Error(err),
				zap.String("hash", hash.String()))
			continue // Skip this document but continue with others
		}

		// Write document to target repository vault
		if err := storeStreamToVault(&buf, vaultDir, hash); err != nil {
			GlobalLogger.Warn("failed to store document to target vault during sync",
				zap.Error(err),
				zap.String("hash", hash.String()))
			continue // Skip this document but continue with others
		}

		GlobalLogger.Debug("synced document file from remote",
			zap.String("hash", hash.String()),
			zap.String("mime_type", docInfo.MIMEType))
	}

	return nil
}

// processBucketDataFromRemote processes bucket data received from remote repository and writes to local database.
// If datasetIDSet is not nil, filters data to only process items related to the specified datasets.
func (r *localFullRepository) processBucketDataFromRemote(ctx context.Context, bd *bboltproto.SyncToBucketData, documentHashesToSync map[string]bool, datasetIDSet map[uuid.UUID]struct{}) error {
	bucketName := bd.BucketName
	key := bd.Key
	value := bd.Value

	return r.db.Update(func(tx *bbolt.Tx) error {
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
			}
			if bd.IsBucket && len(bd.SubKey) > 0 {
				if subBucket.Get(bd.SubKey) == nil {
					if err := subBucket.Put(bd.SubKey, bd.SubValue); err != nil {
						return err
					}
				}
			}
		case "ds":
			// Filter by datasetIDSet if provided
			if datasetIDSet != nil {
				dsID := uuid.UUID(key)
				if _, ok := datasetIDSet[dsID]; !ok {
					return nil // Dataset not in the set, skip
				}
			}

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
				// Update BleveIndex if master branch exists
				if r.config.bleveIndexUpdater != nil {
					r.config.bleveIndexUpdater.updateAfterSync(ctx, r, uuid.UUID(key), DefaultBranch)
				}
			} else if bd.IsBucket && len(bd.SubKey) > 0 {
				// Check if identical or merge
				existingValue := dsBucket.Get(bd.SubKey)
				if existingValue == nil {
					if err := dsBucket.Put(bd.SubKey, bd.SubValue); err != nil {
						return err
					}
				} else if !bytes.Equal(existingValue, bd.SubValue) {
					// TO BE SPECIFIED: For Datasets with different content, perform detailed analysis
					// of all revisions and all Named Branches and their parents.
					// This should include:
					// - Comparing all Dataset revisions between source and target
					// - Analyzing Named Branch assignments and their commit histories
					// - Determining merge strategy based on branch relationships
					// - Handling conflicts and divergence scenarios
					// Current implementation: simplified merge (keep existing if different)
				}
				// Update BleveIndex if master branch exists (only if master branch was updated)
				if r.config.bleveIndexUpdater != nil && bytes.HasPrefix(bd.SubKey, []byte{byte(refBranchPrefix)}) {
					// Check if this is the master branch
					masterBranchKey := bytesToRefKey(stringAsImmutableBytes(DefaultBranch), refBranchPrefix)
					if bytes.Equal(bd.SubKey, masterBranchKey) {
						r.config.bleveIndexUpdater.updateAfterSync(ctx, r, uuid.UUID(key), DefaultBranch)
					}
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
			}
			if bd.IsBucket && len(bd.SubKey) > 0 {
				if commitBucket.Get(bd.SubKey) == nil {
					if err := commitBucket.Put(bd.SubKey, bd.SubValue); err != nil {
						return err
					}
				} else if !bytes.Equal(commitBucket.Get(bd.SubKey), bd.SubValue) {
					// TO BE SPECIFIED: For Commits with different content, perform detailed comparison
					// of Named Branches. This should include:
					// - Comparing branch structures and commit histories
					// - Analyzing parent commit relationships
					// - Determining merge points and divergence
					// - Handling branch conflicts and reconciliation strategies
					// Current implementation: keep existing (simplified)
				}
			}
		case "document_info":
			// If datasetIDSet is provided, skip syncing documents
			if datasetIDSet != nil {
				return nil // Skip document sync when specific datasets are specified
			}

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

// isCommitForDatasets checks if a Commit contains any of the specified datasets.
// It does this by checking if the commit bucket contains entries for the specified datasets.
func isCommitForDatasets(
	commitHash Hash,
	datasetIDSet map[uuid.UUID]struct{},
	commitBucket *bbolt.Bucket,
) bool {
	if datasetIDSet == nil {
		return true // No filter, include all
	}

	found := false

	// Check if commit contains any of the specified datasets
	// Key format in commit: \x00 + dataset UUID, value is DatasetRevision hash
	commitBucket.ForEach(func(k, v []byte) error {
		if found {
			return nil // Already found, skip
		}
		if len(k) > 1 && k[0] == commitDsPrefix {
			dsID := uuid.UUID(k[1:])
			if _, ok := datasetIDSet[dsID]; ok {
				found = true
				return nil
			}
		}
		return nil
	})

	return found
}

// isCommitInBranch checks if a commit belongs to the specified branch.
// It checks if any dataset in datasetIDSet (or all datasets if nil) has the branch pointing to this commit.
func isCommitInBranch(
	commitHash Hash,
	branchName string,
	datasetIDSet map[uuid.UUID]struct{},
	datasetsBucket *bbolt.Bucket,
) bool {
	if isAllBranches(branchName) {
		return true // All branches, include all commits
	}

	found := false
	branchKey := bytesToRefKey(stringAsImmutableBytes(branchName), refBranchPrefix)

	// Check all datasets or only specified ones
	datasetsBucket.ForEach(func(k, v []byte) error {
		if found {
			return nil // Already found, skip
		}

		// Filter by datasetIDSet if provided
		if datasetIDSet != nil {
			dsID := uuid.UUID(k)
			if _, ok := datasetIDSet[dsID]; !ok {
				return nil // Dataset not in the set, skip
			}
		}

		// Check if this dataset has the branch pointing to the commit
		dsBucket := datasetsBucket.Bucket(k)
		if dsBucket == nil {
			return nil
		}

		branchCommitBytes := dsBucket.Get(branchKey)
		if branchCommitBytes != nil && bytes.Equal(branchCommitBytes, commitHash[:]) {
			found = true
			return nil
		}

		return nil
	})

	return found
}

// syncCommits copies Commits from source to target if not present.
// If datasetIDSet is not nil, only syncs Commits that contain the specified datasets.
// If branchName is not empty, only syncs Commits from the specified branch.
func (r *localFullRepository) syncCommits(ctx context.Context, from *localFullRepository, datasetIDSet map[uuid.UUID]struct{}, branchName string) error {
	return from.db.View(func(fromTx *bbolt.Tx) error {
		fromBucket := fromTx.Bucket(keyCommits)
		if fromBucket == nil {
			return nil // Source has no Commits bucket
		}

		// Get datasetsBucket for branch filtering
		var datasetsBucket *bbolt.Bucket
		if !isAllBranches(branchName) {
			datasetsBucket = fromTx.Bucket(keyDatasets)
		}

		return r.db.Update(func(toTx *bbolt.Tx) error {
			toBucket, err := toTx.CreateBucketIfNotExists(keyCommits)
			if err != nil {
				return err
			}

			return fromBucket.ForEach(func(k, v []byte) error {
				// k is the Commit hash, v is a sub-bucket
				fromSubBucket := fromBucket.Bucket(k)
				if fromSubBucket == nil {
					return nil // Not a bucket, skip
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
					if !isCommitForDatasets(commitHash, datasetIDSet, fromSubBucket) {
						return nil // Not for specified datasets, skip
					}
				}

				// Check if already exists in target
				if toBucket.Bucket(k) != nil {
					// Check if content is identical
					toSubBucket := toBucket.Bucket(k)
					if r.commitsAreIdentical(fromSubBucket, toSubBucket) {
						return nil // Identical, skip
					}
					// Different content - for now, we'll keep the target's version
					// This behavior may need to be specified more clearly
					return nil
				}

				// Create sub-bucket in target
				toSubBucket, err := toBucket.CreateBucket(k)
				if err != nil {
					return err
				}

				// Copy all entries from sub-bucket
				return fromSubBucket.ForEach(func(sk, sv []byte) error {
					return toSubBucket.Put(sk, sv)
				})
			})
		})
	})
}

// commitsAreIdentical checks if two commit buckets have identical content.
func (r *localFullRepository) commitsAreIdentical(bucket1, bucket2 *bbolt.Bucket) bool {
	seen := make(map[string][]byte)
	err := bucket1.ForEach(func(k, v []byte) error {
		seen[string(k)] = v
		return nil
	})
	if err != nil {
		return false
	}

	identical := true
	err = bucket2.ForEach(func(k, v []byte) error {
		if val, ok := seen[string(k)]; !ok || !bytes.Equal(val, v) {
			identical = false
		}
		delete(seen, string(k))
		return nil
	})
	if err != nil {
		return false
	}

	return identical && len(seen) == 0
}

// syncDocumentInfo copies document_info from source to target.
// If datasetIDSet is nil, syncs all document_info.
// If datasetIDSet is not nil, skips syncing documents (not implemented yet).
func (r *localFullRepository) syncDocumentInfo(ctx context.Context, from *localFullRepository, datasetIDSet map[uuid.UUID]struct{}) error {
	// If datasetIDSet is provided, skip syncing documents for now
	// TODO: Implement proper filtering to sync only documents referenced by the specified datasets
	if datasetIDSet != nil {
		GlobalLogger.Debug("Skipping document sync when specific datasets are specified (not yet implemented)")
		return nil
	}

	// Collect all document hashes that need to be synced
	var hashesToSync []Hash
	err := from.db.View(func(fromTx *bbolt.Tx) error {
		fromBucket := fromTx.Bucket([]byte(documentInfoBucket))
		if fromBucket == nil {
			return nil // Source has no document_info bucket
		}

		return r.db.View(func(toTx *bbolt.Tx) error {
			toBucket := toTx.Bucket([]byte(documentInfoBucket))
			if toBucket == nil {
				// Target has no document_info bucket, sync all
				return fromBucket.ForEach(func(k, v []byte) error {
					fromSubBucket := fromBucket.Bucket(k)
					if fromSubBucket == nil {
						return nil // Not a bucket, skip
					}
					// Parse hash from key (k is hash as string)
					hash, err := StringToHash(string(k))
					if err != nil {
						return nil // Skip invalid hash
					}
					hashesToSync = append(hashesToSync, hash)
					return nil
				})
			}

			return fromBucket.ForEach(func(k, v []byte) error {
				// k is the document hash (as string), v is a sub-bucket
				fromSubBucket := fromBucket.Bucket(k)
				if fromSubBucket == nil {
					return nil // Not a bucket, skip
				}

				// Check if already exists in target
				if toBucket.Bucket(k) != nil {
					return nil // Already exists, skip
				}

				// Parse hash from key
				hash, err := StringToHash(string(k))
				if err != nil {
					return nil // Skip invalid hash
				}
				hashesToSync = append(hashesToSync, hash)
				return nil
			})
		})
	})
	if err != nil {
		return err
	}

	// Sync metadata first
	err = from.db.View(func(fromTx *bbolt.Tx) error {
		fromBucket := fromTx.Bucket([]byte(documentInfoBucket))
		if fromBucket == nil {
			return nil // Source has no document_info bucket
		}

		return r.db.Update(func(toTx *bbolt.Tx) error {
			toBucket, err := toTx.CreateBucketIfNotExists([]byte(documentInfoBucket))
			if err != nil {
				return err
			}

			return fromBucket.ForEach(func(k, v []byte) error {
				// k is the document hash (as string), v is a sub-bucket
				fromSubBucket := fromBucket.Bucket(k)
				if fromSubBucket == nil {
					return nil // Not a bucket, skip
				}

				// Check if already exists in target
				if toBucket.Bucket(k) != nil {
					return nil // Already exists, skip
				}

				// Create sub-bucket in target
				toSubBucket, err := toBucket.CreateBucket(k)
				if err != nil {
					return err
				}

				// Copy all entries from sub-bucket
				return fromSubBucket.ForEach(func(sk, sv []byte) error {
					return toSubBucket.Put(sk, sv)
				})
			})
		})
	})
	if err != nil {
		return err
	}

	// Sync actual files for each document hash
	for _, hash := range hashesToSync {
		// Check if file already exists in target
		vaultDir := filepath.Join(r.config.repositoryDir, VaultDirName)
		fullPath := vaultPath(vaultDir, hash)
		if _, err := os.Stat(fullPath); err == nil {
			// File already exists, skip
			continue
		}

		// Read document from source repository
		var buf bytes.Buffer
		docInfo, err := from.Document(ctx, hash, &buf)
		if err != nil {
			GlobalLogger.Warn("failed to read document from source repository during sync",
				zap.Error(err),
				zap.String("hash", hash.String()))
			continue // Skip this document but continue with others
		}

		// Write document to target repository vault
		if err := storeStreamToVault(&buf, vaultDir, hash); err != nil {
			GlobalLogger.Warn("failed to store document to target vault during sync",
				zap.Error(err),
				zap.String("hash", hash.String()))
			continue // Skip this document but continue with others
		}

		GlobalLogger.Debug("synced document file",
			zap.String("hash", hash.String()),
			zap.String("mime_type", docInfo.MIMEType))
	}

	return nil
}

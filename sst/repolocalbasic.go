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
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/goccy/go-json"

	"github.com/semanticstep/sst-core/sstauth"

	"github.com/blevesearch/bleve/v2"
	"github.com/google/uuid"
	fs "github.com/relab/wrfs"
	"go.etcd.io/bbolt"
	"go.uber.org/zap"
)

var (
	errRepositoryBucketDoesNotExist = errors.New("repository bucket does not exist")
	rootRepositoryBucketName        = ([]byte)("repository")
)

type localBasicRepository struct {
	stateControl
	url                  *url.URL
	config               repoConfig
	db                   *bbolt.DB
	repositoryBucketName []byte
	bleveIndex           bleve.Index        // getter method Bleve()
	accessRight          sstauth.AccessMode // fixed at open/create time from filesystem probe
}

func (r *localBasicRepository) SuperRepository() SuperRepository {
	return nil
}

func (r *localBasicRepository) URL() string {
	return r.url.String()
}

func (r *localBasicRepository) OpenStage(mode TriplexMode) Stage {
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

func (r *localBasicRepository) RegisterIndexHandler(sdi *SSTDeriveInfo) error {
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
			return wrapError(fmt.Errorf("failed to remove bleve index: %w", err))
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
				return wrapError(fmt.Errorf("failed to rebuild bleve index: %w", err))
			}
		}
	} else {
		// If there is no index folder before, create a new one
		if err := r.buildBleveIndex(); err != nil {
			return wrapError(fmt.Errorf("failed to create bleve index: %w", err))
		}
	}
	return nil
}

func (r *localBasicRepository) buildBleveIndex() error {
	indexDir := filepath.Join(r.config.repositoryDir, indexBleveDir)
	bleveInfoPath := filepath.Join(indexDir, bleveInfoName)

	// Check if the directory exists
	if _, err := os.Stat(indexDir); os.IsNotExist(err) {
		// Directory does not exist, create it
		if err := os.MkdirAll(indexDir, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create bleve index directory %s: %w", indexDir, err)
		}
		GlobalLogger.Info("bleve index directory created", zap.String("path", indexDir))
	} else if err != nil {
		return fmt.Errorf("failed to stat bleve index directory %s: %w", indexDir, err)
	} else {
		// Directory exists, removeAll
		GlobalLogger.Info("removing existing bleve index directory", zap.String("path", indexDir))
		if err := os.RemoveAll(indexDir); err != nil {
			return fmt.Errorf("failed to remove old bleve index directory %s: %w", indexDir, err)
		}
		GlobalLogger.Info("old bleve index directory removed", zap.String("path", indexDir))
	}

	// creates a new bleve index
	bi, err := bleve.New(indexDir, r.config.deriveInfo.DefaultIndexMapping())
	if err != nil {
		return fmt.Errorf("failed to create bleve index: %w", err)
	}
	//  create JSON data for saving deriveInfo
	jsonData, err := json.MarshalIndent(r.config.deriveInfo, "", "    ")
	if err != nil {
		bi.Close()
		return fmt.Errorf("failed to marshal derive info: %w", err)
	}

	// Create a JSON file
	file, err := os.Create(bleveInfoPath)
	if err != nil {
		bi.Close()
		return fmt.Errorf("failed to create bleve info file %s: %w", bleveInfoPath, err)
	}
	defer file.Close()

	// put data into JSON file
	_, err = file.Write(jsonData)
	if err != nil {
		bi.Close()
		return fmt.Errorf("failed to write bleve info file %s: %w", bleveInfoPath, err)
	}

	// set PreCommitCondition for update
	var batchUpdateBatched sync.Map
	r.config.preCommitCondition = func(stage Stage) (postCommitNotifier, error) {
		pcn, err := stagePreCommitConstraints(r, bi, &batchUpdateBatched, stage)
		if err != nil {
			return nil, err
		}
		if pcn != nil {
			return pcn, nil
		}
		return nil, nil
	}

	r.bleveIndex = newHealthAwareIndex(bi)

	// Use the default asynchronous Bleve updater.
	r.config.bleveIndexUpdater = newDefaultBleveIndexUpdater(r.bleveIndex.(*healthAwareIndex))

	if err := r.view(func(rFS fs.FS) error {
		return rebuildIndex(context.Background(), r, bi, rFS)
	}); err != nil {
		bi.Close()
		return fmt.Errorf("failed to rebuild index: %w", err)
	}

	return nil
}

func (r *localBasicRepository) Bleve() bleve.Index {
	if err := r.enter(); err != nil {
		GlobalLogger.Error("failed to get bleve", zap.Error(err))
		return nil
	}
	defer r.leave()

	return r.bleveIndex
}

// RebuildBleveIndex closes the current index, removes the index directory,
// and rebuilds it from scratch.
func (r *localBasicRepository) RebuildBleveIndex() error {
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

func (r *localBasicRepository) DatasetIDs(ctx context.Context) ([]uuid.UUID, error) {
	if err := r.enter(); err != nil {
		return nil, err
	}
	defer r.leave()

	var datasets []uuid.UUID

	err := r.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(r.repositoryBucketName)
		if bucket == nil {
			return fmt.Errorf("repository bucket not found")
		}

		return bucket.ForEach(func(k, v []byte) error {
			id, err := uuid.FromBytes(k)
			if err != nil {
				return fmt.Errorf("invalid dataset ID: %w", err)
			}

			datasets = append(datasets, id)
			return nil
		})
	})
	if err != nil {
		return nil, err
	}

	return datasets, nil
}

func (r *localBasicRepository) Datasets(ctx context.Context) ([]IRI, error) {
	if err := r.enter(); err != nil {
		return nil, err
	}
	defer r.leave()

	var datasetIRIs []IRI
	var datasetIDs []uuid.UUID

	err := r.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(r.repositoryBucketName)
		if bucket == nil {
			return fmt.Errorf("repository bucket not found")
		}

		return bucket.ForEach(func(k, v []byte) error {
			id, err := uuid.FromBytes(k)
			if err != nil {
				return fmt.Errorf("invalid dataset ID: %w", err)
			}

			datasetIDs = append(datasetIDs, id)
			return nil
		})
	})

	err = r.view(func(stageFS fs.FS) error {
		for _, val := range datasetIDs {
			var dgFile fs.File
			dgFile, err = stageFS.Open(val.String())
			if err != nil {
				return fmt.Errorf("failed to open dataset file %s: %w", val.String(), err)
			}
			defer dgFile.Close()
			// read current Dataset's main NG and referenced NG into stage
			iri, err := sstReadGraphURI(bufio.NewReader(dgFile))
			if err != nil {
				return fmt.Errorf("failed to read graph URI from dataset file %s: %w", val.String(), err)
			}
			iri = strings.TrimSuffix(iri, "#")
			datasetIRIs = append(datasetIRIs, IRI(iri))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return datasetIRIs, nil
}

func (r *localBasicRepository) ForDatasets(ctx context.Context, c func(ds Dataset) error) error {
	return r.view(func(stageFS fs.FS) error {
		dirEntries, err := fs.ReadDir(stageFS, ".")
		if err != nil {
			return err
		}

		for _, entry := range dirEntries {
			id, err := uuid.Parse(entry.Name())
			if err != nil {
				continue
			}

			dgFile, err := stageFS.Open(entry.Name())
			if err != nil {
				return err
			}
			iri, err := sstReadGraphURI(bufio.NewReader(dgFile))
			dgFile.Close()
			if err != nil {
				return err
			}
			iri = strings.TrimSuffix(iri, "#")
			ds := &localBasicDataset{r, id, IRI(iri)}

			if err := c(ds); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *localBasicRepository) ForDocuments(ctx context.Context, c func(d DocumentInfo) error) error {
	return notSupported("ForDocuments")
}

func (r *localBasicRepository) datasetByID(ctx context.Context, id uuid.UUID) (di Dataset, err error) {
	if err := r.enter(); err != nil {
		return nil, err
	}
	defer r.leave()

	var iri string
	err = r.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(r.repositoryBucketName)
		key, value := bucket.Cursor().Seek(id[:])
		if !bytes.Equal(key, id[:]) {
			return ErrDatasetNotFound
		}
		// Read IRI from the SST file stored in the bucket
		var err error
		iri, err = sstReadGraphURI(bufio.NewReader(bytes.NewReader(value)))
		if err != nil {
			return err
		}
		iri = strings.TrimSuffix(iri, "#")
		return nil
	})
	if err != nil {
		return
	}
	return &localBasicDataset{r, id, IRI(iri)}, err
}

func (r *localBasicRepository) Dataset(ctx context.Context, iri IRI) (di Dataset, err error) {
	id, err := iriToUUID(iri)
	if err != nil {
		return nil, err
	}
	return r.datasetByID(ctx, id)
}

func (r *localBasicRepository) CommitDetails(ctx context.Context, hashes []Hash) ([]*CommitDetails, error) {
	return nil, fmt.Errorf("CommitDetails not supported in localBasicRepository")
}

func (r *localBasicRepository) CheckoutCommit(ctx context.Context, commitID Hash, mode TriplexMode) (Stage, error) {
	return nil, wrapError(ErrNotSupported)
}

func (r *localBasicRepository) Close() error {
	// mark stateClosing - CompareAndSwap will make sure only marked once
	r.state.CompareAndSwap(int32(stateOpen), int32(stateClosing))

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

func (r *localBasicRepository) view(f func(rFS fs.FS) error) error {
	if err := r.enter(); err != nil {
		return err
	}
	defer r.leave()

	return r.db.View(func(tx *bbolt.Tx) error {
		repositoryBucket := tx.Bucket(r.repositoryBucketName)
		if repositoryBucket == nil {
			return errRepositoryBucketDoesNotExist
		}
		return f(localBasicFsOf(repositoryBucket))
	})
}

func (r *localBasicRepository) update(f func(rFS fs.FS) error) error {
	if err := r.enter(); err != nil {
		return err
	}
	defer r.leave()

	return r.db.Update(func(tx *bbolt.Tx) error {
		repositoryBucket, err := tx.CreateBucketIfNotExists(r.repositoryBucketName)
		if err != nil {
			return err
		}
		return f(localBasicFsOf(repositoryBucket))
	})
}

type localBasicDataset struct {
	r   *localBasicRepository
	id  uuid.UUID
	iri IRI
}

func (d *localBasicDataset) FindCommonParentRevision(context context.Context, revision1, revision2 Hash) (parent Hash, err error) {
	return HashNil(), ErrNotSupported
}

func (d *localBasicDataset) IRI() IRI {
	return d.iri
}

func (d *localBasicDataset) SetBranchCommit(ctx context.Context, commit Hash, branch string) error {
	return wrapError(ErrNotSupported)
}

func (d *localBasicDataset) SetBranchRevision(ctx context.Context, datasetRevision Hash, branch string) error {
	return wrapError(ErrNotSupported)
}

func (d *localBasicDataset) RemoveBranch(ctx context.Context, branch string) error {
	return wrapError(ErrNotSupported)
}

func (d *localBasicDataset) ID() uuid.UUID {
	return d.id
}

func (d *localBasicDataset) CheckoutBranch(ctx context.Context, b string, mode TriplexMode) (Stage, error) {
	var err error
	var st Stage

	err = d.r.view(func(stageFS fs.FS) error {
		datasetID := d.id
		var dgFile fs.File
		dgFile, err = stageFS.Open(datasetID.String())
		if err != nil {
			return fmt.Errorf("failed to open dataset file %s: %w", datasetID.String(), err)
		}
		defer dgFile.Close()
		// read current Dataset's main NG and referenced NG into stage
		ng, err := SstRead(bufio.NewReader(dgFile), DefaultTriplexMode)
		if err != nil {
			return fmt.Errorf("failed to read SST file %s: %w", datasetID.String(), err)
		}

		st = ng.Stage()
		st.(*stage).repo = d.r

		alreadyLoaded := make(map[uuid.UUID]struct{})

		if err := getAllImportsForLocalBasicRepo(stageFS, ng, st, alreadyLoaded); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, wrapError(err)
	}

	return st, nil
}

func getAllImportsForLocalBasicRepo(stageDir fs.FS, ng NamedGraph, s Stage, alreadyLoaded map[uuid.UUID]struct{}) error {
	for _, di := range ng.DirectImports() {
		var dgFile fs.File
		dgFile, err := stageDir.Open(di.ID().String())
		if err != nil {
			return wrapError(fmt.Errorf("failed to open import file %s: %w", di.ID().String(), err))
		}
		defer dgFile.Close()

		ng, err := SstRead(bufio.NewReader(dgFile), DefaultTriplexMode)
		if err != nil {
			return wrapError(fmt.Errorf("failed to read import file %s: %w", di.ID().String(), err))
		}

		// merge temp info into main
		if _, found := alreadyLoaded[di.ID()]; !found {
			if _, err := s.MoveAndMerge(context.TODO(), ng.Stage()); err != nil {
				return wrapError(fmt.Errorf("failed to merge import %s: %w", di.ID().String(), err))
			}
			alreadyLoaded[di.ID()] = struct{}{}
		}

		// run this recursively
		if err := getAllImportsForLocalBasicRepo(stageDir, s.NamedGraph(di.IRI()), s, alreadyLoaded); err != nil {
			return err
		}
	}
	return nil
}

func (d *localBasicDataset) Repository() Repository {
	if err := d.r.enter(); err != nil {
		GlobalLogger.Error("failed to get Repository", zap.Error(err))
		return nil
	}
	defer d.r.leave()

	return d.r
}

func (d *localBasicDataset) CheckoutCommit(ctx context.Context, commitID Hash, mode TriplexMode) (Stage, error) {
	return nil, wrapError(ErrNotSupported)
}

func (d *localBasicDataset) CheckoutRevision(ctx context.Context, datasetRevision Hash, mode TriplexMode) (Stage, error) {
	return nil, wrapError(ErrNotSupported)
}

func (d *localBasicDataset) CommitDetailsByHash(
	ctx context.Context,
	commitID Hash,
) (*CommitDetails, error) {
	return nil, wrapError(ErrNotSupported)
}

func (d *localBasicDataset) CommitDetailsByBranch(
	ctx context.Context,
	branch string,
) (*CommitDetails, error) {
	return nil, wrapError(ErrNotSupported)
}

func (d *localBasicDataset) CommitForRevision(
	ctx context.Context,
	datasetRevision Hash,
) (Hash, error) {
	return HashNil(), wrapError(ErrNotSupported)
}

func (d *localBasicDataset) Branches(ctx context.Context) (map[string]Hash, error) {
	return nil, wrapError(ErrNotSupported)
}

func (d *localBasicDataset) LeafCommits(ctx context.Context) ([]Hash, error) {
	return nil, nil
}

func (r *localBasicRepository) commitNewVersion(ctx context.Context, stage *stage, message string, branch string) (Hash, []uuid.UUID, error) {
	if err := r.enter(); err != nil {
		return emptyHash, nil, err
	}
	defer r.leave()

	if message == "" {
		return Hash{}, nil, ErrEmptyCommitMessage
	}
	var pcNotifier postCommitNotifier
	if r.config.preCommitCondition != nil {
		pcn, err := r.config.preCommitCondition(stage)
		if err != nil {
			return Hash{}, nil, fmt.Errorf("%w: %s", ErrPreCommitConditionFailed, err.Error())
		}
		pcNotifier = pcn
	}
	err := r.update(func(rFS fs.FS) error {
		return stage.writeToSstFilesWithID(rFS)
	})
	if err != nil {
		return Hash{}, nil, err
	}

	modifiedDatasets := make([]uuid.UUID, 0)
	for key := range stage.modifiedDatasets() {
		modifiedDatasets = append(modifiedDatasets, key)
	}

	if r.config.bleveIndexUpdater != nil {
		r.config.bleveIndexUpdater.updateAfterCommit(ctx, r, stage, pcNotifier, commitInfo{}, modifiedDatasets, branch)
	}
	if r.config.postCommitNotification != nil {
		r.config.postCommitNotification(stage, HashNil())
	}

	return HashNil(), modifiedDatasets, nil
}

// calculateRepositoryVersionHash is not supported for localBasicRepository.
func (r *localBasicRepository) calculateRepositoryVersionHash() (string, error) {
	return "", nil // Return empty string to indicate not supported
}

func (r *localBasicRepository) Info(ctx context.Context, branchName string) (RepositoryInfo, error) {
	if err := r.enter(); err != nil {
		return RepositoryInfo{}, err
	}
	defer r.leave()

	// Calculate the total size of the Bbolt database
	dbSize, err := calculateBboltDBSize(r.db)
	if err != nil {
		return RepositoryInfo{}, fmt.Errorf("failed to calculate bbolt DB size: %w", err)
	}

	// Count the number of datasets in the repository bucket
	numDatasets, err := countKeysInBucket(r.db, r.repositoryBucketName)
	if err != nil {
		return RepositoryInfo{}, fmt.Errorf("failed to count datasets: %w", err)
	}

	// Calculate the size of the Bleve index
	var bleveSize int64
	if r.bleveIndex != nil {
		bleveSize, err = calculateBleveIndexSize(r.config.repositoryDir)
		if err != nil {
			return RepositoryInfo{}, fmt.Errorf("failed to calculate bleve index size: %w", err)
		}
	}

	// Document DB size (not supported in localBasicRepository)
	var DocumentDBSize int64

	// Number of documents (not supported in localBasicRepository)
	var NumberOfDocuments int64

	// Calculate version hash
	VersionHash, err := r.calculateRepositoryVersionHash()
	if err != nil {
		return RepositoryInfo{}, err
	}

	// Return repository statistics
	return RepositoryInfo{
		URL: r.url.String(),
		// AccessRight was fixed at open/create time and never changes
		// during the lifetime of the repository instance.
		AccessRight:                 r.accessRight,
		MasterDBSize:                dbSize,
		DerivedDBSize:               bleveSize,
		DocumentDBSize:              DocumentDBSize,
		ActualRepositorySize:        dbSize,
		MaxRepositorySize:           0,
		NumberOfDatasets:            numDatasets,
		NumberOfDatasetsInBranch:    numDatasets,
		NumberOfDatasetRevisions:    numDatasets,
		NumberOfNamedGraphRevisions: numDatasets,
		NumberOfCommits:             0,
		NumberOfRepositoryLogs:      0,
		NumberOfDocuments:           NumberOfDocuments,
		IsRemote:                    false,
		SupportRevisionHistory:      false,
		BleveName:                   r.config.deriveInfo.DerivePackageName,
		BleveVersion:                r.config.deriveInfo.DerivePackageVersion,
		VersionHash:                 VersionHash,
	}, nil
}

func (r *localBasicRepository) Log(ctx context.Context, start, end *int) ([]RepositoryLogEntry, error) {
	return nil, errors.New("RepositoryLog not supported in localBasicRepository")
}

func calculateBboltDBSize(db *bbolt.DB) (int64, error) {
	info, err := os.Stat(db.Path())
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

func countKeysInBucket(db *bbolt.DB, bucketName []byte) (int64, error) {
	var count int64
	err := db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(bucketName)
		if bucket == nil {
			return errRepositoryBucketDoesNotExist
		}
		count = 0
		return bucket.ForEach(func(k, v []byte) error {
			count++
			return nil
		})
	})
	return count, err
}

func calculateBleveIndexSize(dir string) (int64, error) {
	blevePath := filepath.Join(dir, indexBleveDir)

	var size int64
	err := filepath.Walk(blevePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

func notSupported(method string) error {
	return fmt.Errorf("%s not supported in localBasicRepository", method)
}

func (r *localBasicRepository) DocumentSet(ctx context.Context, MIMEType string, source *bufio.Reader) (Hash, error) {
	return Hash{}, notSupported("DocumentSet")
}

func (r *localBasicRepository) Document(ctx context.Context, hash Hash, target io.Writer) (*DocumentInfo, error) {
	return nil, notSupported("Document")
}

func (r *localBasicRepository) Documents(ctx context.Context) ([]DocumentInfo, error) {
	return nil, notSupported("Document")
}

func (r *localBasicRepository) DocumentDelete(ctx context.Context, hash Hash) error {
	return notSupported("DocumentDelete")
}

func (r *localBasicRepository) ExtractSstFile(ctx context.Context, namedGraphRevision Hash, w io.Writer) error {
	return notSupported("ExtractSstFile")
}

func (r *localBasicRepository) SyncFrom(ctx context.Context, from Repository, options ...SyncOption) error {
	// Parse options (not used, but required for interface compliance)
	_ = defaultSyncOptions()
	_ = options
	return fmt.Errorf("SyncFrom is not supported for localBasicRepository - only LocalFullRepository and RemoteRepository support SyncFrom")
}

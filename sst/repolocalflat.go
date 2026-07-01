// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/blevesearch/bleve/v2"
	"github.com/google/uuid"
	fs "github.com/relab/wrfs"
	"github.com/semanticstep/sst-core/sstauth"
	"go.uber.org/zap"
)

var errDirectoryExpected = errors.New("directory expected")

type localFlatRepository struct {
	url         *url.URL
	config      repoConfig
	accessRight sstauth.AccessMode // fixed at open/create time from filesystem probe
}

func (r *localFlatRepository) SuperRepository() SuperRepository {
	return nil
}

func (r *localFlatRepository) URL() string {
	return r.url.String()
}

func (r *localFlatRepository) RegisterIndexHandler(*SSTDeriveInfo) error {
	return ErrNotAvailable
}

func (r *localFlatRepository) Bleve() bleve.Index {
	return nil
}

func (r *localFlatRepository) RebuildBleveIndex() error {
	return ErrNotAvailable
}

func (r *localFlatRepository) OpenStage(mode TriplexMode) Stage {
	return &stage{
		repo:                     r,
		localGraphs:              map[uuid.UUID]*namedGraph{},
		referencedGraphs:         map[string]*namedGraph{},
		assignedNamedGraphNumber: 1,
	}
}

func (r *localFlatRepository) Datasets(ctx context.Context) ([]IRI, error) {
	var datasetIRIs []IRI

	err := filepath.Walk(r.config.repositoryDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(info.Name()) == ".sst" {
			filename := strings.TrimSuffix(info.Name(), ".sst")
			baseURL, err := decodeBaseURL(filename)
			if err != nil {
				// Skip files that don't match the expected format
				return nil
			}
			datasetIRIs = append(datasetIRIs, IRI(baseURL))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return datasetIRIs, nil
}

func (r *localFlatRepository) ForDatasets(ctx context.Context, c func(ds Dataset) error) error {
	return filepath.Walk(r.config.repositoryDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(info.Name()) == ".sst" {
			filename := strings.TrimSuffix(info.Name(), ".sst")
			baseURL, err := decodeBaseURL(filename)
			if err != nil {
				// Skip files that don't match the expected format
				return nil
			}
			ds := &localFlatDataset{r: r, iri: IRI(baseURL)}

			if err := c(ds); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *localFlatRepository) ForDocuments(ctx context.Context, c func(d DocumentInfo) error) error {
	return notSupported("ForDocuments")
}

func (r *localFlatRepository) datasetByID(ctx context.Context, id uuid.UUID) (di Dataset, err error) {
	datasetFileName := filepath.Join(r.config.repositoryDir, id.String()+".sst")
	_, err = os.Stat(datasetFileName)
	if err != nil {
		if os.IsNotExist(err) {
			err = ErrDatasetNotFound
		}
		return
	}
	datasetFile, err := os.Open(datasetFileName)
	if err != nil {
		return
	}
	defer func() {
		e := datasetFile.Close()
		if err == nil {
			err = e
		}
	}()
	// Read IRI from the SST file
	iri, err := sstReadGraphURI(bufio.NewReader(datasetFile))
	if err != nil {
		return nil, err
	}
	iri = strings.TrimSuffix(iri, "#")
	return &localFlatDataset{r: r, id: id, iri: IRI(iri)}, err
}

func (r *localFlatRepository) Dataset(ctx context.Context, iri IRI) (di Dataset, err error) {
	datasetFileName := r.sstFilePathFromIRI(iri)
	_, err = os.Stat(datasetFileName)
	if err != nil {
		if os.IsNotExist(err) {
			err = ErrDatasetNotFound
		}
		return
	}
	return &localFlatDataset{r: r, iri: iri}, nil
}

func (r *localFlatRepository) CommitDetails(ctx context.Context, hashes []Hash) ([]*CommitDetails, error) {
	return nil, ErrNotSupported
}

func (r *localFlatRepository) CheckoutCommit(ctx context.Context, commitID Hash, mode TriplexMode) (Stage, error) {
	return nil, wrapError(ErrNotSupported)
}

func (r *localFlatRepository) Close() error {
	return nil
}

type localFlatDataset struct {
	r   *localFlatRepository
	id  uuid.UUID
	iri IRI
}

func (d *localFlatDataset) FindCommonParentRevision(context context.Context, revision1, revision2 Hash) (parent Hash, err error) {
	return HashNil(), ErrNotSupported
}

func (d *localFlatDataset) IRI() IRI {
	if d.iri != "" {
		return d.iri
	}
	return IRI(fmt.Sprintf("urn:uuid:%s#", d.id.String()))
}

func (d *localFlatDataset) SetBranchCommit(ctx context.Context, commit Hash, branch string) error {
	return wrapError(ErrNotSupported)
}

func (d *localFlatDataset) SetBranchRevision(ctx context.Context, datasetRevision Hash, branch string) error {
	return wrapError(ErrNotSupported)
}

func (d *localFlatDataset) RemoveBranch(ctx context.Context, branch string) error {
	return wrapError(ErrNotSupported)
}

func (d *localFlatDataset) FetchFromRepository() (map[string]Hash, error) {
	return nil, wrapError(ErrNotSupported)
}

func (d *localFlatDataset) ID() uuid.UUID {
	return d.id
}

func (d *localFlatDataset) Branches(ctx context.Context) (map[string]Hash, error) {
	return nil, wrapError(ErrNotSupported)
}

func (d *localFlatDataset) LeafCommits(ctx context.Context) ([]Hash, error) {
	return nil, nil
}

func (d *localFlatDataset) CheckoutBranch(ctx context.Context, b string, mode TriplexMode) (Stage, error) {
	fp := d.r.sstFilePathFromIRI(d.IRI())
	dgFile, err := os.Open(fp)
	if err != nil {
		return nil, wrapError(fmt.Errorf("failed to open SST file %s: %w", fp, err))
	}
	defer dgFile.Close()

	ng, err := SstRead(bufio.NewReader(dgFile), DefaultTriplexMode)
	if err != nil {
		return nil, wrapError(fmt.Errorf("failed to read SST file %s: %w", fp, err))
	}

	st := ng.Stage()
	// set the repo
	st.(*stage).repo = d.r

	alreadyLoaded := make(map[uuid.UUID]struct{})

	if err := getAllImportsForLocalFlatRepo(ng, st, alreadyLoaded); err != nil {
		return nil, err
	}

	return st, nil
}

func getAllImportsForLocalFlatRepo(ng NamedGraph, s Stage, alreadyLoaded map[uuid.UUID]struct{}) error {
	repo := s.(*stage).repo.(*localFlatRepository)
	for _, di := range ng.DirectImports() {
		fp := repo.sstFilePathFromIRI(di.IRI())
		dgFile, err := os.Open(fp)
		if err != nil {
			return wrapError(fmt.Errorf("failed to open import file %s: %w", fp, err))
		}
		defer dgFile.Close()

		ng, err := SstRead(bufio.NewReader(dgFile), DefaultTriplexMode)
		if err != nil {
			return wrapError(fmt.Errorf("failed to read import file %s: %w", fp, err))
		}

		// merge temp info into main
		if _, found := alreadyLoaded[di.ID()]; !found {
			if _, err := s.MoveAndMerge(context.TODO(), ng.Stage()); err != nil {
				return wrapError(fmt.Errorf("failed to merge import %s: %w", di.ID().String(), err))
			}
			alreadyLoaded[di.ID()] = struct{}{}
		}

		// run this recursively
		if err := getAllImportsForLocalFlatRepo(s.NamedGraph(di.IRI()), s, alreadyLoaded); err != nil {
			return err
		}
	}
	return nil
}

func (d *localFlatDataset) Repository() Repository {
	return d.r
}

func (d *localFlatDataset) CheckoutCommit(ctx context.Context, commitID Hash, mode TriplexMode) (Stage, error) {
	return nil, wrapError(fmt.Errorf("CheckoutCommit not supported in localFlatRepository"))
}

func (d *localFlatDataset) CheckoutRevision(ctx context.Context, datasetRevision Hash, mode TriplexMode) (Stage, error) {
	return nil, wrapError(fmt.Errorf("CheckoutRevision not supported in localFlatRepository"))
}

func (d *localFlatDataset) CommitDetailsByHash(
	ctx context.Context,
	commitID Hash,
) (*CommitDetails, error) {
	return nil, wrapError(fmt.Errorf("CommitDetailsByHash not supported in localFlatRepository"))
}

func (d *localFlatDataset) CommitDetailsByBranch(
	ctx context.Context,
	branch string,
) (*CommitDetails, error) {
	return nil, wrapError(fmt.Errorf("CommitDetailsByBranch not supported in localFlatRepository"))
}

func (d *localFlatDataset) CommitForRevision(
	ctx context.Context,
	datasetRevision Hash,
) (Hash, error) {
	return HashNil(), wrapError(fmt.Errorf("CommitForRevision not supported in localFlatRepository"))
}

func (r *localFlatRepository) commitNewVersion(ctx context.Context, stage *stage, message string, branch string) (Hash, []uuid.UUID, error) {
	if message == "" {
		return Hash{}, nil, ErrEmptyCommitMessage
	}

	err := stage.WriteSstFilesDirectory(fs.DirFS(r.config.repositoryDir))
	if err != nil {
		return Hash{}, nil, err
	}

	if r.config.postCommitNotification != nil {
		var revision Hash
		r.config.postCommitNotification(stage, revision)
	}

	modifiedDatasets := make([]uuid.UUID, 0)
	for key := range stage.modifiedDatasets() {
		modifiedDatasets = append(modifiedDatasets, key)
	}

	return HashNil(), modifiedDatasets, nil
}

func (r *localFlatRepository) calculateRepositoryVersionHash() (string, error) {
	type fileEntry struct {
		absPath string
		relPath string
	}

	entries := make([]fileEntry, 0)
	err := filepath.WalkDir(r.config.repositoryDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(d.Name()), ".sst") {
			rel, err := filepath.Rel(r.config.repositoryDir, path)
			if err != nil {
				return err
			}
			entries = append(entries, fileEntry{
				absPath: path,
				relPath: filepath.ToSlash(rel),
			})
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("failed to walk repository directory: %w", err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].relPath < entries[j].relPath
	})

	hasher := sha256.New()
	for _, e := range entries {
		_, _ = hasher.Write([]byte(e.relPath))
		_, _ = hasher.Write([]byte{0})

		f, err := os.Open(e.absPath)
		if err != nil {
			return "", fmt.Errorf("failed to open SST file %q: %w", e.relPath, err)
		}
		_, copyErr := io.Copy(hasher, f)
		closeErr := f.Close()
		if copyErr != nil {
			return "", fmt.Errorf("failed to read SST file %q: %w", e.relPath, copyErr)
		}
		if closeErr != nil {
			return "", fmt.Errorf("failed to close SST file %q: %w", e.relPath, closeErr)
		}
		_, _ = hasher.Write([]byte{0})
	}

	return BytesToHash(hasher.Sum(nil)).String(), nil
}

func (r *localFlatRepository) Info(ctx context.Context, branchName string) (RepositoryInfo, error) {
	// Calculate the total size of all .sst files in the repository directory
	totalSize, err := calculateSSTFilesSize(r.config.repositoryDir)
	if err != nil {
		return RepositoryInfo{}, fmt.Errorf("failed to calculate total .sst file size: %w", err)
	}

	// Count the number of .sst files in the directory
	numDatasets, err := countSSTFilesInDirectory(r.config.repositoryDir)
	if err != nil {
		return RepositoryInfo{}, fmt.Errorf("failed to count .sst files: %w", err)
	}

	// Document DB size (not supported in localFlatRepository)
	var DocumentDBSize int64

	// Number of documents (not supported in localFlatRepository)
	var NumberOfDocuments int64

	// Calculate version hash (not supported)
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
		MasterDBSize:                totalSize, // Total size of all .sst files
		DerivedDBSize:               0,
		DocumentDBSize:              DocumentDBSize,
		ActualRepositorySize:        totalSize,
		MaxRepositorySize:           0,
		NumberOfDatasets:            numDatasets, // Number of .sst files
		NumberOfDatasetsInBranch:    numDatasets, // Same as NumDatasets
		NumberOfDatasetRevisions:    numDatasets, // Same as NumDatasets
		NumberOfNamedGraphRevisions: numDatasets, // Same as NumDatasets
		NumberOfCommits:             0,
		NumberOfRepositoryLogs:      0,
		NumberOfDocuments:           NumberOfDocuments,
		IsRemote:                    false,
		SupportRevisionHistory:      false,
		BleveName:                   "",
		BleveVersion:                "",
		VersionHash:                 VersionHash,
	}, nil
}

func (r *localFlatRepository) Log(ctx context.Context, start, end *int) ([]RepositoryLogEntry, error) {
	return nil, errors.New("RepositoryLog not supported in localFlatRepository")
}

// Helper: Calculate the total size of all .sst files in the given directory.
func calculateSSTFilesSize(dir string) (int64, error) {
	var totalSize int64
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			GlobalLogger.Error("error accessing path while calculating SST size", zap.String("path", path), zap.Error(err))
			return nil
		}
		if info.IsDir() {
			return nil // Skip directories
		}
		if strings.HasSuffix(info.Name(), ".sst") {
			totalSize += info.Size()
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("error walking directory: %w", err)
	}
	return totalSize, nil
}

// Helper: Count the number of .sst files in the given directory.
func countSSTFilesInDirectory(dir string) (int64, error) {
	var count int64
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			GlobalLogger.Error("error accessing path while counting SST files", zap.String("path", path), zap.Error(err))
			return nil
		}
		if info.IsDir() {
			return nil // Skip directories
		}
		if strings.HasSuffix(info.Name(), ".sst") {
			count++
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("error walking directory: %w", err)
	}
	return count, nil
}

func (r localFlatRepository) DocumentSet(ctx context.Context, MIMEType string, source *bufio.Reader) (Hash, error) {
	return Hash{}, notSupported("DocumentSet")
}

func (r localFlatRepository) Document(ctx context.Context, hash Hash, target io.Writer) (*DocumentInfo, error) {
	return nil, notSupported("Document")
}

func (r localFlatRepository) Documents(ctx context.Context) ([]DocumentInfo, error) {
	return nil, notSupported("Documents")
}

func (r localFlatRepository) DocumentDelete(ctx context.Context, hash Hash) error {
	return notSupported("DocumentDelete")
}

func (r localFlatRepository) ExtractSstFile(ctx context.Context, namedGraphRevision Hash, w io.Writer) error {
	return notSupported("ExtractSstFile")
}

func (r *localFlatRepository) SyncFrom(ctx context.Context, from Repository, options ...SyncOption) error {
	// Parse options (not used, but required for interface compliance)
	_ = defaultSyncOptions()
	_ = options
	return fmt.Errorf("SyncFrom is not supported for localFlatRepository - only LocalFullRepository and RemoteRepository support SyncFrom")
}

// sstFilePathFromIRI returns the SST file path for a given IRI.
// The filename is the base64URL-encoded base URL portion of the IRI.
func (r *localFlatRepository) sstFilePathFromIRI(iri IRI) string {
	baseURL, _ := iri.Split()
	filename := encodeBaseURL(baseURL) + ".sst"
	return filepath.Join(r.config.repositoryDir, filename)
}

// decodeBaseURL decodes a base64 URL-encoded string back to the original base URL.
// Returns an error if the string is not a valid base64 URL encoding.
func decodeBaseURL(encoded string) (string, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}

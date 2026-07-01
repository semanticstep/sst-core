// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst

import (
	"bufio"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/blevesearch/bleve/v2"
	"github.com/google/uuid"
	fs "github.com/relab/wrfs"
)

type localFlatFileSystemRepository struct {
	fs fs.FS
}

func (r *localFlatFileSystemRepository) SuperRepository() SuperRepository {
	return nil
}

func (r *localFlatFileSystemRepository) URL() string {
	return ""
}

func (r *localFlatFileSystemRepository) RegisterIndexHandler(*SSTDeriveInfo) error {
	return ErrNotAvailable
}

func (r *localFlatFileSystemRepository) Bleve() bleve.Index {
	return nil
}

func (r *localFlatFileSystemRepository) RebuildBleveIndex() error {
	return ErrNotAvailable
}

func (r *localFlatFileSystemRepository) OpenStage(mode TriplexMode) Stage {
	return &stage{
		repo:                     r,
		localGraphs:              map[uuid.UUID]*namedGraph{},
		referencedGraphs:         map[string]*namedGraph{},
		assignedNamedGraphNumber: 1,
	}
}

func (r *localFlatFileSystemRepository) Datasets(ctx context.Context) ([]IRI, error) {
	var datasetIRIs []IRI

	err := fs.WalkDir(r.fs, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("failed to get file info for %s: %w", path, err)
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

func (r *localFlatFileSystemRepository) ForDatasets(ctx context.Context, c func(ds Dataset) error) error {
	return fs.WalkDir(r.fs, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		info, err := d.Info()
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
			ds := &localFlatFileSystemDataset{r: r, iri: IRI(baseURL)}

			if err := c(ds); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *localFlatFileSystemRepository) ForDocuments(ctx context.Context, c func(d DocumentInfo) error) error {
	return notSupported("ForDocuments")
}

func (r *localFlatFileSystemRepository) datasetByID(ctx context.Context, id uuid.UUID) (di Dataset, err error) {
	datasetFileName := id.String() + ".sst"
	datasetFile, err := r.fs.Open(datasetFileName)
	if err != nil {
		return
	}
	defer datasetFile.Close()

	// Read IRI from the SST file
	iri, err := sstReadGraphURI(bufio.NewReader(datasetFile))
	if err != nil {
		return nil, err
	}
	iri = strings.TrimSuffix(iri, "#")
	return &localFlatFileSystemDataset{r: r, id: id, iri: IRI(iri)}, err
}

func (r *localFlatFileSystemRepository) Dataset(ctx context.Context, iri IRI) (di Dataset, err error) {
	datasetFileName := sstFileNameFromIRI(iri)
	_, err = r.fs.Open(datasetFileName)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, ErrDatasetNotFound
		}
		return nil, err
	}
	return &localFlatFileSystemDataset{r: r, iri: iri}, nil
}

func (r *localFlatFileSystemRepository) CommitDetails(ctx context.Context, hashes []Hash) ([]*CommitDetails, error) {
	return nil, fmt.Errorf("CommitDetails not supported in localFlatFileSystemRepository")
}

func (r *localFlatFileSystemRepository) CheckoutCommit(ctx context.Context, commitID Hash, mode TriplexMode) (Stage, error) {
	return nil, wrapError(ErrNotSupported)
}

func (r *localFlatFileSystemRepository) Close() error {
	return nil
}

type localFlatFileSystemDataset struct {
	r   *localFlatFileSystemRepository
	id  uuid.UUID
	iri IRI
}

func (d localFlatFileSystemDataset) FindCommonParentRevision(context context.Context, revision1, revision2 Hash) (parent Hash, err error) {
	return HashNil(), ErrNotSupported
}

func (d *localFlatFileSystemDataset) IRI() IRI {
	if d.iri != "" {
		return d.iri
	}
	return IRI(fmt.Sprintf("urn:uuid:%s", d.id.String()))
}

func (d *localFlatFileSystemDataset) SetBranchCommit(ctx context.Context, commit Hash, branch string) error {
	return wrapError(fmt.Errorf("SetBranch not supported in localFlatFileSystemRepository"))
}

func (d *localFlatFileSystemDataset) SetBranchRevision(ctx context.Context, datasetRevision Hash, branch string) error {
	return wrapError(fmt.Errorf("SetBranchRevision not supported in localFlatFileSystemRepository"))
}

func (d *localFlatFileSystemDataset) RemoveBranch(ctx context.Context, branch string) error {
	return wrapError(fmt.Errorf("RemoveBranch not supported in localFlatFileSystemRepository"))
}

func (d *localFlatFileSystemDataset) FetchFromRepository() (map[string]Hash, error) {
	return nil, wrapError(fmt.Errorf("FetchFromRepository not supported in localFlatFileSystemRepository"))
}

func (d *localFlatFileSystemDataset) ID() uuid.UUID {
	return d.id
}

func (d *localFlatFileSystemDataset) Branches(ctx context.Context) (map[string]Hash, error) {
	return nil, wrapError(fmt.Errorf("Branches not supported in localFlatFileSystemRepository"))
}

func (d *localFlatFileSystemDataset) LeafCommits(ctx context.Context) ([]Hash, error) {
	return nil, nil
}

func (d *localFlatFileSystemDataset) CheckoutBranch(ctx context.Context, b string, mode TriplexMode) (Stage, error) {
	datasetFileName := sstFileNameFromIRI(d.IRI())
	datasetFile, err := d.r.fs.Open(datasetFileName)
	if err != nil {
		return nil, err
	}
	defer datasetFile.Close()

	ng, err := SstRead(bufio.NewReader(datasetFile), DefaultTriplexMode)
	if err != nil {
		return nil, wrapError(fmt.Errorf("failed to read SST file %s: %w", datasetFileName, err))
	}

	st := ng.Stage()
	// set the repo
	st.(*stage).repo = d.r

	alreadyLoaded := make(map[uuid.UUID]struct{})

	if err := getAllImportsForLocalFlatFileSystemRepo(d.r.fs, ng, st, alreadyLoaded); err != nil {
		return nil, err
	}

	return st, nil
}

func getAllImportsForLocalFlatFileSystemRepo(fsys fs.FS, ng NamedGraph, s Stage, alreadyLoaded map[uuid.UUID]struct{}) error {
	for _, di := range ng.DirectImports() {
		fp := sstFileNameFromIRI(di.IRI())
		dgFile, err := fsys.Open(fp)
		if err != nil {
			return wrapError(fmt.Errorf("failed to open import file %s: %w", fp, err))
		}

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
		if err := getAllImportsForLocalFlatFileSystemRepo(fsys, s.NamedGraph(di.IRI()), s, alreadyLoaded); err != nil {
			return err
		}
	}
	return nil
}

func (d *localFlatFileSystemDataset) Repository() Repository {
	return d.r
}

func (d *localFlatFileSystemDataset) CheckoutCommit(ctx context.Context, commitID Hash, mode TriplexMode) (Stage, error) {
	return nil, wrapError(fmt.Errorf("CheckoutCommit not supported in localFlatFileSystemRepository"))
}

func (d *localFlatFileSystemDataset) CheckoutRevision(ctx context.Context, datasetRevision Hash, mode TriplexMode) (Stage, error) {
	return nil, wrapError(fmt.Errorf("CheckoutRevision not supported in localFlatFileSystemRepository"))
}

func (d *localFlatFileSystemDataset) CommitDetailsByHash(
	ctx context.Context,
	commitID Hash,
) (*CommitDetails, error) {
	return nil, wrapError(fmt.Errorf("CommitDetailsByHash not supported in localFlatFileSystemRepository"))
}

func (d *localFlatFileSystemDataset) CommitDetailsByBranch(
	ctx context.Context,
	branch string,
) (*CommitDetails, error) {
	return nil, wrapError(fmt.Errorf("CommitDetailsByBranch not supported in localFlatFileSystemRepository"))
}

func (d *localFlatFileSystemDataset) CommitForRevision(
	ctx context.Context,
	datasetRevision Hash,
) (Hash, error) {
	return HashNil(), wrapError(fmt.Errorf("CommitForRevision not supported in localFlatFileSystemRepository"))
}

func (r *localFlatFileSystemRepository) commitNewVersion(ctx context.Context, stage *stage, message string, branch string) (Hash, []uuid.UUID, error) {
	return Hash{}, nil, wrapError(fmt.Errorf("Commit not supported in localFlatFileSystemRepository"))
}

func (r *localFlatFileSystemRepository) calculateRepositoryVersionHash() (string, error) {
	type fileEntry struct {
		openPath string
		relSlash string
	}

	var entries []fileEntry
	err := fs.WalkDir(r.fs, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.EqualFold(filepath.Ext(d.Name()), ".sst") {
			return nil
		}
		entries = append(entries, fileEntry{openPath: path, relSlash: filepath.ToSlash(path)})
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("failed to walk repository filesystem: %w", err)
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].relSlash < entries[j].relSlash })

	hasher := sha256.New()
	for _, e := range entries {
		_, _ = hasher.Write([]byte(e.relSlash))
		_, _ = hasher.Write([]byte{0})
		f, err := r.fs.Open(e.openPath)
		if err != nil {
			return "", fmt.Errorf("failed to open SST file %q: %w", e.relSlash, err)
		}
		_, copyErr := io.Copy(hasher, f)
		closeErr := f.Close()
		if copyErr != nil {
			return "", fmt.Errorf("failed to read SST file %q: %w", e.relSlash, copyErr)
		}
		if closeErr != nil {
			return "", fmt.Errorf("failed to close SST file %q: %w", e.relSlash, closeErr)
		}
		_, _ = hasher.Write([]byte{0})
	}

	return BytesToHash(hasher.Sum(nil)).String(), nil
}

func (r *localFlatFileSystemRepository) Info(ctx context.Context, branchName string) (RepositoryInfo, error) {
	// // Calculate the total size of all .sst files in the repository directory
	// totalSize, err := calculateSSTFilesSize(r.config.repositoryDir)
	// if err != nil {
	// 	return RepositoryInfo{}, fmt.Errorf("failed to calculate total .sst file size: %w", err)
	// }

	// // Count the number of .sst files in the directory
	// numDatasets, err := countSSTFilesInDirectory(r.config.repositoryDir)
	// if err != nil {
	// 	return RepositoryInfo{}, fmt.Errorf("failed to count .sst files: %w", err)
	// }

	// // Return repository statistics
	// return RepositoryInfo{
	// 	MasterDBSize:                totalSize, // Total size of all .sst files
	// 	DerivedDBSize:               0,
	// 	NumberOfDatasets:            numDatasets, // Number of .sst files
	// 	NumberOfDatasetRevisions:    numDatasets, // Same as NumDatasets
	// 	NumberOfNamedGraphRevisions: numDatasets, // Same as NumDatasets
	// 	NumberOfCommits:             0,
	// 	NumberOfRepositoryLogs:         0,
	// 	IsRemote:                    false,
	// 	SupportRevisionHistory:      false,
	// }, nil

	versionHash, err := r.calculateRepositoryVersionHash()
	if err != nil {
		return RepositoryInfo{}, err
	}
	return RepositoryInfo{VersionHash: versionHash}, nil
}

func (r *localFlatFileSystemRepository) Log(ctx context.Context, start, end *int) ([]RepositoryLogEntry, error) {
	return nil, errors.New("RepositoryLog not supported in localFlatFileSystemRepository")
}

func (r localFlatFileSystemRepository) DocumentSet(ctx context.Context, MIMEType string, source *bufio.Reader) (Hash, error) {
	return Hash{}, notSupported("DocumentSet")
}

func (r localFlatFileSystemRepository) Document(ctx context.Context, hash Hash, target io.Writer) (*DocumentInfo, error) {
	return nil, notSupported("Document")
}

func (r localFlatFileSystemRepository) Documents(ctx context.Context) ([]DocumentInfo, error) {
	return nil, notSupported("Documents")
}

func (r localFlatFileSystemRepository) DocumentDelete(ctx context.Context, hash Hash) error {
	return notSupported("DocumentDelete")
}

func (r localFlatFileSystemRepository) ExtractSstFile(ctx context.Context, namedGraphRevision Hash, w io.Writer) error {
	return notSupported("ExtractSstFile")
}

func (r *localFlatFileSystemRepository) SyncFrom(ctx context.Context, from Repository, options ...SyncOption) error {
	// Parse options (not used, but required for interface compliance)
	_ = defaultSyncOptions()
	_ = options
	return fmt.Errorf("SyncFrom is not supported for localFlatFileSystemRepository - only LocalFullRepository and RemoteRepository support SyncFrom")
}

// sstFileNameFromIRI returns the SST filename for a given IRI.
// The filename is the base64URL-encoded base URL portion of the IRI.
func sstFileNameFromIRI(iri IRI) string {
	baseURL, _ := iri.Split()
	return encodeBaseURL(baseURL) + ".sst"
}

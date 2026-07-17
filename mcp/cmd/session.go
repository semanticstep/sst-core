// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package mcp

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/semanticstep/sst-core/cli/cmd/utils"
	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/tools/validate"
	"github.com/google/uuid"
)

// Session holds MCP-scoped SST resource handles.
type Session struct {
	mu sync.Mutex

	repoCounter  int
	repos        map[string]sst.Repository
	repoPaths    map[string]string
	repoTypes    map[string]string
	repoOrder    []string
	authContexts map[sst.Repository]context.Context

	datasetCounter int
	datasets       map[string]sst.Dataset
	datasetRepos   map[string]string
	datasetIRIs    map[string]string
	datasetOrder   []string

	stageCounter   int
	stages         map[string]sst.Stage
	stageRepos     map[string]string
	stageBranches  map[string]string
	stageCommits   map[string]string
	stageRevisions map[string]string
	stageOrder     []string
}

// NewSession creates an empty MCP session.
func NewSession() *Session {
	return &Session{
		repos:          make(map[string]sst.Repository),
		repoPaths:      make(map[string]string),
		repoTypes:      make(map[string]string),
		authContexts:   make(map[sst.Repository]context.Context),
		datasets:       make(map[string]sst.Dataset),
		datasetRepos:   make(map[string]string),
		datasetIRIs:    make(map[string]string),
		stages:         make(map[string]sst.Stage),
		stageRepos:     make(map[string]string),
		stageBranches:  make(map[string]string),
		stageCommits:   make(map[string]string),
		stageRevisions: make(map[string]string),
	}
}

func (s *Session) reserveRepoAlias(alias string) (id string, autoID bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id = alias
	autoID = alias == ""
	if autoID {
		id = fmt.Sprintf("r%d", s.repoCounter+1)
	}
	if _, exists := s.repos[id]; exists {
		return "", false, fmt.Errorf("repository alias %q already in use", id)
	}
	return id, autoID, nil
}

func (s *Session) commitRepo(id string, autoID bool, location, repoType string, repo sst.Repository, authCtx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if autoID {
		s.repoCounter++
	}
	s.repos[id] = repo
	s.repoPaths[id] = location
	s.repoTypes[id] = repoType
	if authCtx != nil {
		s.authContexts[repo] = authCtx
	}
	s.repoOrder = append(s.repoOrder, id)
}

// OpenLocalRepository opens a local SST repository and registers it in the session.
func (s *Session) OpenLocalRepository(path, alias string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path is required")
	}

	id, autoID, err := s.reserveRepoAlias(alias)
	if err != nil {
		return "", err
	}

	repo, err := sst.OpenLocalRepository(path, "default@semanticstep.net", "default")
	if err != nil {
		return "", err
	}

	s.commitRepo(id, autoID, path, "local", repo, nil)
	return id, nil
}

// CloseRepository closes one repository by session alias (CLI: <repo>.close).
func (s *Session) CloseRepository(repoAlias string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	repo, ok := s.repos[repoAlias]
	if !ok {
		return fmt.Errorf("repository %q not found", repoAlias)
	}
	if err := repo.Close(); err != nil {
		return err
	}
	delete(s.authContexts, repo)
	delete(s.repos, repoAlias)
	delete(s.repoPaths, repoAlias)
	delete(s.repoTypes, repoAlias)
	for i, id := range s.repoOrder {
		if id == repoAlias {
			s.repoOrder = append(s.repoOrder[:i], s.repoOrder[i+1:]...)
			break
		}
	}
	s.removeDatasetsForRepoLocked(repoAlias)
	s.removeStagesForRepoLocked(repoAlias)
	return nil
}

// CloseAll closes all repositories in the session.
func (s *Session) CloseAll() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var firstErr error
	for id, repo := range s.repos {
		if err := repo.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		delete(s.authContexts, repo)
		delete(s.repos, id)
		delete(s.repoPaths, id)
		delete(s.repoTypes, id)
	}
	s.repoOrder = nil
	s.datasets = make(map[string]sst.Dataset)
	s.datasetRepos = make(map[string]string)
	s.datasetIRIs = make(map[string]string)
	s.datasetOrder = nil
	s.stages = make(map[string]sst.Stage)
	s.stageRepos = make(map[string]string)
	s.stageBranches = make(map[string]string)
	s.stageCommits = make(map[string]string)
	s.stageRevisions = make(map[string]string)
	s.stageOrder = nil
	return firstErr
}

// Repository returns a repository by session alias.
func (s *Session) Repository(alias string) (sst.Repository, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	repo, ok := s.repos[alias]
	if !ok {
		return nil, fmt.Errorf("repository %q not found; open a repository first", alias)
	}
	return repo, nil
}

// RepoStatus describes one open repository in the session.
type RepoStatus struct {
	RepoAlias string `json:"repo_alias"`
	Type     string `json:"type"`
	Location string `json:"location"`
}

// DatasetStatus describes one open dataset in the session.
type DatasetStatus struct {
	DatasetAlias string `json:"dataset_alias"`
	RepoAlias    string `json:"repo_alias"`
	IRI          string `json:"iri"`
}

// StageStatus describes one open stage in the session.
type StageStatus struct {
	StageAlias string `json:"stage_alias"`
	RepoAlias  string `json:"repo_alias"`
	Branch     string `json:"branch,omitempty"`
	Commit     string `json:"commit,omitempty"`
	Revision   string `json:"revision,omitempty"`
}

// StatusOutput is the structured status of the MCP session.
type StatusOutput struct {
	Repositories []RepoStatus    `json:"repositories"`
	Datasets     []DatasetStatus `json:"datasets"`
	Stages       []StageStatus   `json:"stages"`
}

// Status returns currently open repositories, datasets, and stages.
func (s *Session) Status() StatusOutput {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := StatusOutput{
		Repositories: make([]RepoStatus, 0, len(s.repoOrder)),
		Datasets:     make([]DatasetStatus, 0, len(s.datasetOrder)),
		Stages:       make([]StageStatus, 0, len(s.stageOrder)),
	}
	for _, id := range s.repoOrder {
		out.Repositories = append(out.Repositories, RepoStatus{
			RepoAlias: id,
			Type:      s.repoTypes[id],
			Location:  s.repoPaths[id],
		})
	}
	for _, id := range s.datasetOrder {
		out.Datasets = append(out.Datasets, DatasetStatus{
			DatasetAlias: id,
			RepoAlias:    s.datasetRepos[id],
			IRI:          s.datasetIRIs[id],
		})
	}
	for _, id := range s.stageOrder {
		out.Stages = append(out.Stages, StageStatus{
			StageAlias: id,
			RepoAlias:  s.stageRepos[id],
			Branch:     s.stageBranches[id],
			Commit:     s.stageCommits[id],
			Revision:   s.stageRevisions[id],
		})
	}
	return out
}

func (s *Session) reserveDatasetAlias(alias string) (id string, autoID bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id = alias
	autoID = alias == ""
	if autoID {
		id = fmt.Sprintf("d%d", s.datasetCounter+1)
	}
	if _, exists := s.datasets[id]; exists {
		return "", false, fmt.Errorf("dataset alias %q already in use", id)
	}
	return id, autoID, nil
}

func (s *Session) commitDataset(id string, autoID bool, repoAlias, iri string, ds sst.Dataset) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if autoID {
		s.datasetCounter++
	}
	s.datasets[id] = ds
	s.datasetRepos[id] = repoAlias
	s.datasetIRIs[id] = iri
	s.datasetOrder = append(s.datasetOrder, id)
}

func (s *Session) removeDatasetsForRepoLocked(repoAlias string) {
	var remaining []string
	for _, id := range s.datasetOrder {
		if s.datasetRepos[id] == repoAlias {
			delete(s.datasets, id)
			delete(s.datasetRepos, id)
			delete(s.datasetIRIs, id)
			continue
		}
		remaining = append(remaining, id)
	}
	s.datasetOrder = remaining
}

func (s *Session) reserveStageAlias(alias string) (id string, autoID bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id = alias
	autoID = alias == ""
	if autoID {
		id = fmt.Sprintf("s%d", s.stageCounter+1)
	}
	if _, exists := s.stages[id]; exists {
		return "", false, fmt.Errorf("stage alias %q already in use", id)
	}
	return id, autoID, nil
}

// stageMeta holds optional checkout metadata recorded with a stage (CLI StageBranches/Commits/Revisions).
type stageMeta struct {
	Branch   string
	Commit   string
	Revision string
}

func (s *Session) commitStage(id string, autoID bool, repoAlias string, stage sst.Stage, meta stageMeta) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if autoID {
		s.stageCounter++
	}
	s.stages[id] = stage
	s.stageRepos[id] = repoAlias
	if meta.Branch != "" {
		s.stageBranches[id] = meta.Branch
	}
	if meta.Commit != "" {
		s.stageCommits[id] = meta.Commit
	}
	if meta.Revision != "" {
		s.stageRevisions[id] = meta.Revision
	}
	s.stageOrder = append(s.stageOrder, id)
}

func (s *Session) removeStagesForRepoLocked(repoAlias string) {
	var remaining []string
	for _, id := range s.stageOrder {
		if s.stageRepos[id] == repoAlias {
			delete(s.stages, id)
			delete(s.stageRepos, id)
			delete(s.stageBranches, id)
			delete(s.stageCommits, id)
			delete(s.stageRevisions, id)
			continue
		}
		remaining = append(remaining, id)
	}
	s.stageOrder = remaining
}

// Stage returns a stage by session alias.
func (s *Session) Stage(alias string) (sst.Stage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	stage, ok := s.stages[alias]
	if !ok {
		return nil, fmt.Errorf("stage %q not found; checkout or open a stage first", alias)
	}
	return stage, nil
}

// OpenDataset opens a dataset by IRI and registers it in the session (CLI: <repo>.dataset <iri>).
func (s *Session) OpenDataset(repoAlias, iri, alias string) (string, error) {
	if iri == "" {
		return "", fmt.Errorf("iri is required")
	}

	repo, err := s.Repository(repoAlias)
	if err != nil {
		return "", err
	}

	id, autoID, err := s.reserveDatasetAlias(alias)
	if err != nil {
		return "", err
	}

	ctx := s.authContextFor(repo)
	ds, err := repo.Dataset(ctx, sst.IRI(iri))
	if err != nil {
		return "", err
	}

	s.commitDataset(id, autoID, repoAlias, iri, ds)
	return id, nil
}

// Dataset returns a dataset by session alias.
func (s *Session) Dataset(alias string) (sst.Dataset, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ds, ok := s.datasets[alias]
	if !ok {
		return nil, fmt.Errorf("dataset %q not found; open a dataset first", alias)
	}
	return ds, nil
}

// ListDatasets returns dataset IRIs for the given repository alias.
func (s *Session) ListDatasets(repoAlias string) ([]string, error) {
	repo, err := s.Repository(repoAlias)
	if err != nil {
		return nil, err
	}

	ctx := s.authContextFor(repo)
	iris, err := repo.Datasets(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]string, 0, len(iris))
	for _, iri := range iris {
		out = append(out, iri.String())
	}
	return out, nil
}

// ListDocuments returns document metadata for the given repository alias.
func (s *Session) ListDocuments(repoAlias string) ([]map[string]any, error) {
	repo, err := s.Repository(repoAlias)
	if err != nil {
		return nil, err
	}

	ctx := s.authContextFor(repo)
	docs, err := repo.Documents(ctx)
	if err != nil {
		return nil, err
	}

	sort.Slice(docs, func(i, j int) bool {
		return docs[i].Timestamp.After(docs[j].Timestamp)
	})

	out := make([]map[string]any, 0, len(docs))
	for _, doc := range docs {
		out = append(out, formatDocumentInfo(doc))
	}
	return out, nil
}

// SetDocument uploads a local file to the repository (CLI: documentset).
func (s *Session) SetDocument(repoAlias, filePath string) (map[string]any, error) {
	if filePath == "" {
		return nil, fmt.Errorf("file_path is required")
	}

	repo, err := s.Repository(repoAlias)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return nil, err
	}

	var hash sst.Hash
	copy(hash[:], hasher.Sum(nil))

	ctx := s.authContextFor(repo)
	docInfo, err := repo.Document(ctx, hash, nil)
	if err == nil {
		result := formatDocumentInfo(*docInfo)
		result["deduplicated"] = true
		result["message"] = "document already exists"
		return result, nil
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	reader := bufio.NewReader(file)
	mimeType := guessMimeType(stat.Name(), reader)

	uploadedHash, err := repo.DocumentSet(ctx, mimeType, reader)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"hash":         uploadedHash.String(),
		"mime_type":    mimeType,
		"deduplicated": false,
		"message":      "document uploaded",
	}, nil
}

// GetDocument downloads a document by hash to a local file (CLI: documentget).
func (s *Session) GetDocument(repoAlias, hashStr, outputPath string) (map[string]any, error) {
	if hashStr == "" {
		return nil, fmt.Errorf("hash is required")
	}

	hash, err := sst.StringToHash(hashStr)
	if err != nil {
		return nil, fmt.Errorf("invalid hash: must be a valid 44-character Base58 string")
	}

	repo, err := s.Repository(repoAlias)
	if err != nil {
		return nil, err
	}

	ctx := s.authContextFor(repo)

	var buf bytes.Buffer
	info, err := repo.Document(ctx, hash, &buf)
	if err != nil {
		return nil, err
	}

	ext := extFromMime(info.MIMEType)
	filename := hash.String() + ext
	savePath := outputPath
	if savePath == "" {
		savePath = filename
	} else if stat, err := os.Stat(savePath); err == nil && stat.IsDir() {
		savePath = filepath.Join(savePath, filename)
	}

	outFile, err := os.Create(savePath)
	if err != nil {
		return nil, err
	}
	defer outFile.Close()

	if _, err := buf.WriteTo(outFile); err != nil {
		return nil, err
	}

	return map[string]any{
		"hash":        hash.String(),
		"output_path": savePath,
		"mime_type":   info.MIMEType,
		"size":        info.Size,
		"message":     "document downloaded",
	}, nil
}

// DocumentInfo returns document metadata without downloading content (CLI: documentinfo).
func (s *Session) DocumentInfo(repoAlias, hashStr string) (map[string]any, error) {
	if hashStr == "" {
		return nil, fmt.Errorf("hash is required")
	}

	hash, err := sst.StringToHash(hashStr)
	if err != nil {
		return nil, fmt.Errorf("invalid hash: must be a valid 44-character Base58 string")
	}

	repo, err := s.Repository(repoAlias)
	if err != nil {
		return nil, err
	}

	ctx := s.authContextFor(repo)
	doc, err := repo.Document(ctx, hash, nil)
	if err != nil {
		return nil, err
	}

	return formatDocumentInfo(*doc), nil
}

// DeleteDocument removes a document by hash from the repository (CLI: documentdelete).
func (s *Session) DeleteDocument(repoAlias, hashStr string) error {
	if hashStr == "" {
		return fmt.Errorf("hash is required")
	}

	hash, err := sst.StringToHash(hashStr)
	if err != nil {
		return fmt.Errorf("invalid hash: must be a valid 44-character Base58 string")
	}

	repo, err := s.Repository(repoAlias)
	if err != nil {
		return err
	}

	ctx := s.authContextFor(repo)
	return repo.DocumentDelete(ctx, hash)
}

// Branches returns branch name → commit hash for a dataset (CLI: <dataset>.branches).
func (s *Session) Branches(datasetAlias string) (map[string]string, error) {
	ds, err := s.Dataset(datasetAlias)
	if err != nil {
		return nil, err
	}

	ctx := s.authContextFor(ds.Repository())
	branches, err := ds.Branches(ctx)
	if err != nil {
		return nil, err
	}

	out := make(map[string]string, len(branches))
	for name, hash := range branches {
		out[name] = hash.String()
	}
	return out, nil
}

// ListCommits walks commit history from leaf commits and branch tips (CLI: <dataset>.listcommits).
// When details is false, each entry is a commit hash string. When true, each entry is commit metadata.
func (s *Session) ListCommits(datasetAlias string, details bool) ([]any, error) {
	ds, err := s.Dataset(datasetAlias)
	if err != nil {
		return nil, err
	}

	ctx := s.authContextFor(ds.Repository())
	roots, err := utils.ListCommitsEntryHashes(ctx, ds)
	if err != nil {
		return nil, err
	}

	visited := make(map[string]bool)
	out := make([]any, 0)
	for _, root := range roots {
		if err := s.walkCommitHistory(ds, root, visited, details, ctx, &out); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (s *Session) walkCommitHistory(
	ds sst.Dataset,
	commit sst.Hash,
	visited map[string]bool,
	details bool,
	ctx context.Context,
	out *[]any,
) error {
	key := commit.String()
	if visited[key] {
		return nil
	}
	visited[key] = true

	commitDetails, err := ds.CommitDetailsByHash(ctx, commit)
	if err != nil {
		return err
	}

	if details {
		*out = append(*out, commitDetailsToMap(commitDetails))
	} else {
		*out = append(*out, key)
	}

	for _, parent := range commitDetails.ParentCommits[ds.IRI()] {
		if err := s.walkCommitHistory(ds, parent, visited, details, ctx, out); err != nil {
			return err
		}
	}
	return nil
}

func commitDetailsToMap(d *sst.CommitDetails) map[string]any {
	datasetRevisions := make(map[string]string, len(d.DatasetRevisions))
	for id, hash := range d.DatasetRevisions {
		datasetRevisions[id.String()] = hash.String()
	}
	namedGraphRevisions := make(map[string]string, len(d.NamedGraphRevisions))
	for id, hash := range d.NamedGraphRevisions {
		namedGraphRevisions[id.String()] = hash.String()
	}
	parentCommits := make(map[string][]string, len(d.ParentCommits))
	for id, parents := range d.ParentCommits {
		list := make([]string, 0, len(parents))
		for _, p := range parents {
			list = append(list, p.String())
		}
		parentCommits[id.String()] = list
	}
	return map[string]any{
		"commit":                d.Commit.String(),
		"author":                d.Author,
		"author_date":           d.AuthorDate.UTC().Format(time.RFC3339),
		"message":               d.Message,
		"dataset_revisions":     datasetRevisions,
		"named_graph_revisions": namedGraphRevisions,
		"parent_commits":        parentCommits,
	}
}

// CommitDetailsByHash returns commit metadata for a hash (CLI: <dataset>.commitdetailsbyhash).
func (s *Session) CommitDetailsByHash(datasetAlias, hashStr string) (map[string]any, error) {
	if hashStr == "" {
		return nil, fmt.Errorf("hash is required")
	}

	ds, err := s.Dataset(datasetAlias)
	if err != nil {
		return nil, err
	}

	hash, err := sst.StringToHash(hashStr)
	if err != nil {
		return nil, fmt.Errorf("invalid commit hash: %w", err)
	}

	ctx := s.authContextFor(ds.Repository())
	details, err := ds.CommitDetailsByHash(ctx, hash)
	if err != nil {
		return nil, err
	}
	return commitDetailsToMap(details), nil
}

// CommitDetailsByBranch returns commit metadata for a branch (CLI: <dataset>.commitdetailsbybranch).
func (s *Session) CommitDetailsByBranch(datasetAlias, branch string) (map[string]any, error) {
	if branch == "" {
		return nil, fmt.Errorf("branch is required")
	}

	ds, err := s.Dataset(datasetAlias)
	if err != nil {
		return nil, err
	}

	ctx := s.authContextFor(ds.Repository())
	details, err := ds.CommitDetailsByBranch(ctx, branch)
	if err != nil {
		return nil, err
	}
	return commitDetailsToMap(details), nil
}

// SyncFrom copies data from sourceRepoAlias into targetRepoAlias (CLI: <repo>.syncfrom).
func (s *Session) SyncFrom(targetRepoAlias, sourceRepoAlias, branch string, datasetRefs []string) (map[string]any, error) {
	if targetRepoAlias == "" {
		return nil, fmt.Errorf("target_repo_alias is required")
	}
	if sourceRepoAlias == "" {
		return nil, fmt.Errorf("source_repo_alias is required")
	}
	if targetRepoAlias == sourceRepoAlias {
		return nil, fmt.Errorf("cannot sync from a repository to itself")
	}

	targetRepo, err := s.Repository(targetRepoAlias)
	if err != nil {
		return nil, err
	}
	sourceRepo, err := s.Repository(sourceRepoAlias)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	targetType := s.repoTypes[targetRepoAlias]
	sourceType := s.repoTypes[sourceRepoAlias]
	s.mu.Unlock()

	ctx := s.authContextFor(targetRepo)
	if targetType != "remote" && sourceType == "remote" {
		ctx = s.authContextFor(sourceRepo)
	}

	var syncOptions []sst.SyncOption
	if branch != "" && branch != "*" {
		syncOptions = append(syncOptions, sst.WithBranch(branch))
	}

	var datasetIRIs []sst.IRI
	if len(datasetRefs) > 0 {
		datasetIRIs, err = s.resolveDatasetIRIs(datasetRefs)
		if err != nil {
			return nil, err
		}
		syncOptions = append(syncOptions, sst.WithDatasetIRIs(datasetIRIs...))
	}

	if err := targetRepo.SyncFrom(ctx, sourceRepo, syncOptions...); err != nil {
		return nil, err
	}

	result := map[string]any{
		"target_repo_alias": targetRepoAlias,
		"source_repo_alias": sourceRepoAlias,
		"message":           "sync completed",
	}
	if branch != "" {
		result["branch"] = branch
	} else {
		result["branch"] = "*"
	}
	if len(datasetIRIs) > 0 {
		iris := make([]string, len(datasetIRIs))
		for i, iri := range datasetIRIs {
			iris[i] = string(iri)
		}
		result["datasets"] = iris
	}
	return result, nil
}

func (s *Session) resolveDatasetIRIs(refs []string) ([]sst.IRI, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	datasetIRIs := make([]sst.IRI, 0, len(refs))
	for _, ref := range refs {
		if ref == "" {
			continue
		}
		if ds, exists := s.datasets[ref]; exists {
			datasetIRIs = append(datasetIRIs, ds.IRI())
			continue
		}
		if id, err := uuid.Parse(ref); err == nil {
			datasetIRIs = append(datasetIRIs, sst.IRI(id.URN()))
			continue
		}
		datasetIRIs = append(datasetIRIs, sst.IRI(ref))
	}
	return datasetIRIs, nil
}

// History returns the commit history graph for a dataset (CLI: <dataset>.history).
// Commits are ordered oldest-first with parent hashes and branch tip labels.
func (s *Session) History(datasetAlias string) ([]map[string]any, error) {
	ds, err := s.Dataset(datasetAlias)
	if err != nil {
		return nil, err
	}

	ctx := s.authContextFor(ds.Repository())
	ngIRI := ds.IRI()

	leafCommits, err := ds.LeafCommits(ctx)
	if err != nil {
		return nil, err
	}
	if len(leafCommits) == 0 {
		branches, berr := ds.Branches(ctx)
		if berr != nil {
			return nil, berr
		}
		for _, h := range branches {
			leafCommits = append(leafCommits, h)
		}
	}
	if len(leafCommits) == 0 {
		return []map[string]any{}, nil
	}

	type histNode struct {
		commit  string
		message string
		author  string
		date    time.Time
		parents []string
	}

	nextLayer := make(map[sst.Hash]struct{})
	for _, h := range leafCommits {
		nextLayer[h] = struct{}{}
	}

	nodes := make([]*histNode, 0)
	seen := make(map[string]*histNode)

	for len(nextLayer) > 0 {
		currentLayer := nextLayer
		nextLayer = make(map[sst.Hash]struct{})

		for h := range currentLayer {
			details, detErr := ds.CommitDetailsByHash(ctx, h)
			if detErr != nil || details == nil {
				continue
			}
			key := details.Commit.String()
			if _, existed := seen[key]; existed {
				continue
			}

			parents := make([]string, 0, len(details.ParentCommits[ngIRI]))
			for _, p := range details.ParentCommits[ngIRI] {
				parents = append(parents, p.String())
				nextLayer[p] = struct{}{}
			}
			node := &histNode{
				commit:  key,
				message: details.Message,
				author:  details.Author,
				date:    details.AuthorDate,
				parents: parents,
			}
			nodes = append(nodes, node)
			seen[key] = node
		}
	}

	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].date.Before(nodes[j].date)
	})

	branches, err := ds.Branches(ctx)
	if err != nil {
		return nil, err
	}
	branchByCommit := make(map[string][]string)
	for name, hash := range branches {
		branchByCommit[hash.String()] = append(branchByCommit[hash.String()], name)
	}
	for _, names := range branchByCommit {
		sort.Strings(names)
	}

	out := make([]map[string]any, 0, len(nodes))
	for _, n := range nodes {
		entry := map[string]any{
			"commit":      n.commit,
			"message":     n.message,
			"author":      n.author,
			"author_date": n.date.UTC().Format(time.RFC3339),
			"parents":     n.parents,
		}
		if tips := branchByCommit[n.commit]; len(tips) > 0 {
			entry["branches"] = tips
		}
		out = append(out, entry)
	}
	return out, nil
}

// CheckoutBranch checks out a branch into a new stage (CLI: <dataset>.checkoutbranch).
func (s *Session) CheckoutBranch(datasetAlias, branchName, stageAlias string) (string, error) {
	if branchName == "" {
		return "", fmt.Errorf("branch is required")
	}

	ds, err := s.Dataset(datasetAlias)
	if err != nil {
		return "", err
	}

	s.mu.Lock()
	repoAlias := s.datasetRepos[datasetAlias]
	s.mu.Unlock()
	if repoAlias == "" {
		return "", fmt.Errorf("dataset %q has no repository association", datasetAlias)
	}

	id, autoID, err := s.reserveStageAlias(stageAlias)
	if err != nil {
		return "", err
	}

	ctx := s.authContextFor(ds.Repository())
	stage, err := ds.CheckoutBranch(ctx, branchName, sst.DefaultTriplexMode)
	if err != nil {
		return "", err
	}

	s.commitStage(id, autoID, repoAlias, stage, stageMeta{Branch: branchName})
	return id, nil
}

// CheckoutCommit checks out a commit into a new stage (CLI: <dataset>.checkoutcommit).
func (s *Session) CheckoutCommit(datasetAlias, hashStr, stageAlias string) (string, error) {
	if hashStr == "" {
		return "", fmt.Errorf("hash is required")
	}

	ds, err := s.Dataset(datasetAlias)
	if err != nil {
		return "", err
	}

	hash, err := sst.StringToHash(hashStr)
	if err != nil {
		return "", fmt.Errorf("invalid commit hash: %w", err)
	}

	s.mu.Lock()
	repoAlias := s.datasetRepos[datasetAlias]
	s.mu.Unlock()
	if repoAlias == "" {
		return "", fmt.Errorf("dataset %q has no repository association", datasetAlias)
	}

	id, autoID, err := s.reserveStageAlias(stageAlias)
	if err != nil {
		return "", err
	}

	ctx := s.authContextFor(ds.Repository())
	stage, err := ds.CheckoutCommit(ctx, hash, sst.DefaultTriplexMode)
	if err != nil {
		return "", err
	}

	s.commitStage(id, autoID, repoAlias, stage, stageMeta{Commit: hash.String()})
	return id, nil
}

// OpenStage creates an empty stage on a repository (CLI: <repo>.openstage).
func (s *Session) OpenStage(repoAlias, stageAlias string) (string, error) {
	repo, err := s.Repository(repoAlias)
	if err != nil {
		return "", err
	}

	id, autoID, err := s.reserveStageAlias(stageAlias)
	if err != nil {
		return "", err
	}

	stage := repo.OpenStage(sst.DefaultTriplexMode)
	s.commitStage(id, autoID, repoAlias, stage, stageMeta{})
	return id, nil
}

// Info returns stage metadata (CLI: <stage>.info).
func (s *Session) Info(stageAlias string) (map[string]any, error) {
	stage, err := s.Stage(stageAlias)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	repoAlias := s.stageRepos[stageAlias]
	branch := s.stageBranches[stageAlias]
	commit := s.stageCommits[stageAlias]
	revision := s.stageRevisions[stageAlias]
	s.mu.Unlock()

	info := stage.Info()

	localGraphs := make([]map[string]any, 0)
	for _, ng := range stage.NamedGraphs() {
		localGraphs = append(localGraphs, map[string]any{
			"iri":           ng.IRI().String(),
			"ibnode_count":  ng.IRINodeCount() + ng.BlankNodeCount(),
		})
	}

	referencedGraphs := make([]map[string]any, 0)
	for _, ng := range stage.ReferencedGraphs() {
		referencedGraphs = append(referencedGraphs, map[string]any{
			"iri":          ng.IRI().String(),
			"ibnode_count": ng.IRINodeCount() + ng.BlankNodeCount(),
		})
	}

	out := map[string]any{
		"stage_alias":                 stageAlias,
		"repo_alias":                  repoAlias,
		"number_of_local_graphs":      info.NumberOfLocalGraphs,
		"local_graphs":                localGraphs,
		"number_of_referenced_graphs": info.NumberOfReferencedGraphs,
		"referenced_graphs":           referencedGraphs,
		"total_number_of_triples":     info.TotalNumberOfTriples,
	}
	if branch != "" {
		out["branch"] = branch
	}
	if commit != "" {
		out["commit"] = commit
	}
	if revision != "" {
		out["revision"] = revision
	}
	return out, nil
}

// NamedGraphs returns local named graph IRIs in a stage (CLI: <stage>.namedgraphs).
func (s *Session) NamedGraphs(stageAlias string) ([]string, error) {
	stage, err := s.Stage(stageAlias)
	if err != nil {
		return nil, err
	}

	graphs := stage.NamedGraphs()
	out := make([]string, 0, len(graphs))
	for _, ng := range graphs {
		out = append(out, ng.IRI().String())
	}
	return out, nil
}

// Validate validates a stage (CLI: <stage>.validate [-o <file>]).
// When outputPath is non-empty, the human-readable report is written there (overwrites).
func (s *Session) Validate(stageAlias, outputPath string) (map[string]any, error) {
	stage, err := s.Stage(stageAlias)
	if err != nil {
		return nil, err
	}

	utils.MuteStdout()
	report, err := validate.Validate(stage, validate.KindRdfType, validate.KindDomainRange)
	utils.RestoreStdout()
	if err != nil {
		return nil, err
	}

	text := report.FormatHumanReadable()
	out := map[string]any{
		"stage_alias": stageAlias,
		"report":      text,
	}

	if outputPath != "" {
		outputPath = utils.EnsureOutputExt(outputPath, ".txt")
		if err := os.WriteFile(outputPath, []byte(text), 0o644); err != nil {
			return nil, err
		}
		out["output_path"] = outputPath
		out["message"] = "validation report written"
	}
	return out, nil
}

// TriG returns the stage RDF as TriG text (CLI: <stage>.trig).
// When outputPath is non-empty, TriG is also written to that file.
func (s *Session) TriG(stageAlias, outputPath string) (map[string]any, error) {
	stage, err := s.Stage(stageAlias)
	if err != nil {
		return nil, err
	}
	if stage == nil {
		return nil, fmt.Errorf("stage %q is nil", stageAlias)
	}
	if !stage.IsValid() {
		return nil, fmt.Errorf("stage %q is not valid", stageAlias)
	}

	var buf bytes.Buffer
	if err := stage.RdfWrite(&buf, sst.RdfFormatTriG); err != nil {
		return nil, err
	}

	text := buf.String()
	out := map[string]any{
		"stage_alias": stageAlias,
		"trig":        text,
	}

	if outputPath != "" {
		outputPath = utils.EnsureOutputExt(outputPath, ".trig")
		if err := os.WriteFile(outputPath, buf.Bytes(), 0o644); err != nil {
			return nil, err
		}
		out["output_path"] = outputPath
		out["message"] = "trig written"
	}
	return out, nil
}

// Commit commits stage changes (CLI: <stage>.commit <message> [branch]).
func (s *Session) Commit(stageAlias, message, branchName string) (map[string]any, error) {
	if strings.TrimSpace(message) == "" {
		return nil, fmt.Errorf("message is required")
	}

	stage, err := s.Stage(stageAlias)
	if err != nil {
		return nil, err
	}
	if stage == nil {
		return nil, fmt.Errorf("stage %q is nil", stageAlias)
	}
	if stage.Repository() == nil {
		return nil, fmt.Errorf("stage %q is not linked to a repository", stageAlias)
	}

	ctx := s.authContextFor(stage.Repository())
	commitHash, datasetIDs, err := stage.Commit(ctx, message, branchName)
	if err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(datasetIDs))
	for _, id := range datasetIDs {
		ids = append(ids, id.String())
	}

	out := map[string]any{
		"stage_alias":  stageAlias,
		"commit":       commitHash.String(),
		"message":      message,
		"dataset_ids":  ids,
	}
	if branchName != "" {
		out["branch"] = branchName
	}
	return out, nil
}

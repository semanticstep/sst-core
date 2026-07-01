// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package mcp

import (
	"context"
	"fmt"
	"sync"

	"github.com/semanticstep/sst-core/sst"
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
}

// NewSession creates an empty MCP session.
func NewSession() *Session {
	return &Session{
		repos:        make(map[string]sst.Repository),
		repoPaths:    make(map[string]string),
		repoTypes:    make(map[string]string),
		authContexts: make(map[sst.Repository]context.Context),
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
	Type      string `json:"type"`
	Location  string `json:"location"`
}

// StatusOutput is the structured status of the MCP session.
type StatusOutput struct {
	Repositories []RepoStatus `json:"repositories"`
}

// Status returns currently open repositories.
func (s *Session) Status() StatusOutput {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := StatusOutput{Repositories: make([]RepoStatus, 0, len(s.repoOrder))}
	for _, id := range s.repoOrder {
		out.Repositories = append(out.Repositories, RepoStatus{
			RepoAlias: id,
			Type:      s.repoTypes[id],
			Location:  s.repoPaths[id],
		})
	}
	return out
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

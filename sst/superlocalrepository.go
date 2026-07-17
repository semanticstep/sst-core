// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/semanticstep/sst-core/bboltproto"
	"github.com/semanticstep/sst-core/bleveproto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

const defaultSuperRepoName = "default"

// SuperRepository is a container that manages multiple related Repository Entries.
// It provides functionality to create, retrieve, list, and delete Repository
// Entries within a single location (local directory or remote server).
//
// A SuperRepository always contains at least one "default" Repository.
// All methods are context-aware for timeout and cancellation support.
// The URL of a Repository Entry in a SuperRepository is constructed by using the SuperRepository's URL as base URL
// and using the name of a Repository Entry as a fragment to the base URL (separated by "#").
type SuperRepository interface {
	// URL returns the specific location where this SuperRepository is stored.
	// For a remote SuperRepository the returned URL uses grpc:// scheme.
	// For a local SuperRepository the returned URL uses file:// scheme.
	URL() string

	// Get retrieves an existing Repository Entry by name from the SuperRepository.
	// If name is empty, it returns the "default" Repository.
	// Returns an error if the Repository does not exist.
	Get(ctx context.Context, name string) (Repository, error)

	// Create creates a new Repository Entry with the given name in the SuperRepository.
	// Returns an error if a Repository Entry with the same name already exists.
	// The created Repository is automatically opened and ready for use.
	Create(ctx context.Context, name string) (Repository, error)

	// Delete removes a Repository Entry by name from the SuperRepository.
	// If name is empty, it deletes the "default" Repository.
	// This operation permanently removes all data in the Repository.
	Delete(ctx context.Context, name string) error

	// List returns the names of all Repository Entries in the SuperRepository.
	// The returned slice is sorted alphabetically.
	List(ctx context.Context) ([]string, error)

	// RegisterIndexHandler registers a Bleve index handler for all Repository
	// Entries in this SuperRepository. Similar to Repository.RegisterIndexHandler but applies
	// to all Repository Entries managed by this SuperRepository.
	// Returns an error if index registration fails.
	RegisterIndexHandler(*SSTDeriveInfo) error

	// GetQuota returns the per-Repository quota for the named Repository Entry.
	GetQuota(ctx context.Context, name string) (RepositoryQuota, error)

	// SetQuota sets the per-Repository quota for the named Repository Entry.
	SetQuota(ctx context.Context, name string, maxSizeBytes int64) error

	// GetTotalQuota returns the aggregate quota across all Repository Entries.
	GetTotalQuota(ctx context.Context) (RepositoryQuota, error)

	// SetTotalQuota sets the aggregate quota across all Repository Entries.
	SetTotalQuota(ctx context.Context, maxSizeBytes int64) error

	// GetMaxRepositoryCount returns the maximum number of Repository Entries, or 0 for unlimited.
	GetMaxRepositoryCount(ctx context.Context) int

	// SetMaxRepositoryCount sets the maximum number of Repository Entries.
	SetMaxRepositoryCount(ctx context.Context, count int) error

	// Close releases all resources, including closing all opened Repository Entries.
	// Any error encountered while closing individual Repository Entries is returned.
	Close() error
}

type localSuperRepository struct {
	rootDir    string
	deriveInfo *SSTDeriveInfo

	mu        sync.RWMutex
	repos     map[string]Repository // opened repo in memory
	known     map[string]struct{}   // repos that stored in the disk
	quotas    quotasConfig
	quotaPath string
}

// quotasConfig is persisted to quotaPath in the SuperRepository root.
type quotasConfig struct {
	Total        quotaEntry            `json:"_total"`
	MaxRepoCount int                   `json:"_maxRepoCount"`
	Repos        map[string]quotaEntry `json:"repos,omitempty"`
}

type quotaEntry struct {
	MaxSizeBytes int64 `json:"maxSizeBytes"`
}

// NewLocalSuperRepository creates or opens a local SuperRepository at the specified directory.
// If the directory does not exist, it will be created with permissions 0755.
// If a SuperRepository already exists at the location, it will be opened.
//
// The function performs the following:
//   - Creates the root directory if it doesn't exist
//   - Scans the directory for existing repositories
//   - Creates a "default" repository if it doesn't exist
//
// Returns an error if the directory cannot be created or accessed.
func NewLocalSuperRepository(rootDir string) (*localSuperRepository, error) {
	if err := os.MkdirAll(rootDir, 0o755); err != nil {
		return nil, fmt.Errorf("create rootDir %q: %w", rootDir, err)
	}

	s := &localSuperRepository{
		rootDir: rootDir,
		repos:   make(map[string]Repository),
		known:   make(map[string]struct{}),
		quotas: quotasConfig{
			Repos: make(map[string]quotaEntry),
		},
	}

	// 2) scan rootDir
	entries, err := os.ReadDir(rootDir)
	if err != nil {
		return nil, fmt.Errorf("readdir %q: %w", rootDir, err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		// only store repo names
		s.known[name] = struct{}{}
	}

	// 3) load quotas
	s.quotaPath = filepath.Join(rootDir, "super-repo-quotas.json")
	if err := s.loadQuotas(); err != nil {
		return nil, fmt.Errorf("load quotas: %w", err)
	}

	// create default repo if not exists
	if _, err = s.Create(context.Background(), defaultSuperRepoName); err != nil &&
		!strings.Contains(err.Error(), "already exists") {
		return nil, err
	}

	return s, nil
}

func (s *localSuperRepository) loadQuotas() error {
	data, err := os.ReadFile(s.quotaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err := json.Unmarshal(data, &s.quotas); err != nil {
		return err
	}
	if s.quotas.Repos == nil {
		s.quotas.Repos = make(map[string]quotaEntry)
	}
	return nil
}

func (s *localSuperRepository) saveQuotasLocked() error {
	data, err := json.MarshalIndent(s.quotas, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.quotaPath, data, 0o600)
}

// calculateRepoSizeLocked returns the on-disk size of the named repository in bytes.
// It accounts for the BBolt database file and the document vault.
// Caller must NOT hold s.mu.
func (s *localSuperRepository) calculateRepoSizeLocked(name string) (int64, error) {
	dir := s.repoDir(name)
	var total int64
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		return nil
	})
	if err != nil {
		return 0, err
	}
	return total, nil
}

func (s *localSuperRepository) repoDir(name string) string {
	return filepath.Join(s.rootDir, name)
}

// URL returns the file:// URL for this local SuperRepository.
func (s *localSuperRepository) URL() string {
	return (&url.URL{Scheme: "file", Path: s.rootDir}).String()
}

func (r *localSuperRepository) RegisterIndexHandler(sd *SSTDeriveInfo) error {
	r.deriveInfo = sd
	return nil
}

func (s *localSuperRepository) Get(ctx context.Context, name string) (Repository, error) {
	if name == "" {
		name = "default"
	}

	// 1) check if stored in repos
	s.mu.RLock()
	repo, ok := s.repos[name]
	s.mu.RUnlock()
	if ok {
		if repo.Bleve() == nil {
			if s.deriveInfo != nil {
				err := repo.RegisterIndexHandler(s.deriveInfo)
				if err != nil {
					return nil, err
				}
			}
		}
		return repo, nil
	}

	// 2) open repo
	dir := s.repoDir(name)
	r, err := OpenLocalRepository(dir, "default@semanticstep.net", name)
	if err != nil {
		if errors.Is(err, ErrRepositoryDoesNotExist) || errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("repository %q does not exist: %w", name, err)
		}
		return nil, fmt.Errorf("open repo %q: %w", name, err)
	}
	lfr := r.(*localFullRepository)
	lfr.sr = s
	lfr.quota = &quotaState{}
	_ = lfr.initDocumentSize()

	if s.deriveInfo != nil {
		err = r.RegisterIndexHandler(s.deriveInfo)
		if err != nil {
			return nil, err
		}
	}
	// put into repos & known
	s.mu.Lock()
	s.repos[name] = r
	s.known[name] = struct{}{}
	s.mu.Unlock()

	return r, nil
}

func (s *localSuperRepository) Create(ctx context.Context, name string) (Repository, error) {
	if name == "" {
		return nil, fmt.Errorf("empty repository name")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.known[name]; ok {
		return nil, fmt.Errorf("repository %q already exists", name)
	}

	// Count quota check.
	if s.quotas.MaxRepoCount > 0 && len(s.known)+1 > s.quotas.MaxRepoCount {
		return nil, ErrQuotaExceeded
	}

	const emptyRepoSize = 32 * 1024

	// Total size quota check (soft).
	if s.quotas.Total.MaxSizeBytes > 0 {
		currentTotal, err := s.calculateTotalSizeLocked()
		if err != nil {
			return nil, fmt.Errorf("calculate total size: %w", err)
		}
		if currentTotal+emptyRepoSize > s.quotas.Total.MaxSizeBytes {
			return nil, ErrQuotaExceeded
		}
	}

	dir := s.repoDir(name)
	r, err := CreateLocalRepository(dir, "default@semanticstep.net", name, true)
	if err != nil {
		return nil, err
	}
	lfr := r.(*localFullRepository)
	lfr.sr = s
	lfr.quota = &quotaState{}
	_ = lfr.initDocumentSize()

	if s.deriveInfo != nil {
		_ = r.RegisterIndexHandler(s.deriveInfo)
	}

	s.repos[name] = r
	s.known[name] = struct{}{}

	return r, nil
}

func (s *localSuperRepository) Delete(ctx context.Context, name string) error {
	if name == "" {
		name = "default"
	}

	s.mu.Lock()
	repo, opened := s.repos[name]
	if opened {
		delete(s.repos, name)
	}
	_, existed := s.known[name]
	if existed {
		delete(s.known, name)
	}
	// Remove any per-repo quota entry for the deleted repository.
	if _, ok := s.quotas.Repos[name]; ok {
		delete(s.quotas.Repos, name)
		_ = s.saveQuotasLocked()
	}
	s.mu.Unlock()

	if opened {
		if closer, ok := repo.(interface{ Close() error }); ok {
			_ = closer.Close()
		}
	}

	dir := s.repoDir(name)
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("remove repo dir %q: %w", dir, err)
	}

	return nil
}

func (s *localSuperRepository) List(ctx context.Context) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// union(known, repos)
	namesSet := make(map[string]struct{}, len(s.known)+len(s.repos))
	for name := range s.known {
		namesSet[name] = struct{}{}
	}
	for name := range s.repos {
		namesSet[name] = struct{}{}
	}

	names := make([]string, 0, len(namesSet))
	for name := range namesSet {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func (s *localSuperRepository) GetQuota(ctx context.Context, name string) (RepositoryQuota, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	q, ok := s.quotas.Repos[name]
	if !ok {
		return RepositoryQuota{}, nil
	}
	actual, err := s.calculateRepoSizeLocked(name)
	if err != nil {
		return RepositoryQuota{}, err
	}
	return RepositoryQuota{MaxSizeBytes: q.MaxSizeBytes, ActualSizeBytes: actual}, nil
}

func (s *localSuperRepository) SetQuota(ctx context.Context, name string, maxSizeBytes int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.quotas.Repos[name] = quotaEntry{MaxSizeBytes: maxSizeBytes}
	return s.saveQuotasLocked()
}

func (s *localSuperRepository) GetTotalQuota(ctx context.Context) (RepositoryQuota, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	actual, err := s.calculateTotalSizeLocked()
	if err != nil {
		return RepositoryQuota{}, err
	}
	return RepositoryQuota{MaxSizeBytes: s.quotas.Total.MaxSizeBytes, ActualSizeBytes: actual}, nil
}

func (s *localSuperRepository) SetTotalQuota(ctx context.Context, maxSizeBytes int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.quotas.Total = quotaEntry{MaxSizeBytes: maxSizeBytes}
	return s.saveQuotasLocked()
}

func (s *localSuperRepository) GetMaxRepositoryCount(ctx context.Context) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.quotas.MaxRepoCount
}

func (s *localSuperRepository) SetMaxRepositoryCount(ctx context.Context, count int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.quotas.MaxRepoCount = count
	return s.saveQuotasLocked()
}

// calculateTotalSizeLocked returns the aggregate on-disk size of all known repositories.
// Caller must hold s.mu for reading.
func (s *localSuperRepository) calculateTotalSizeLocked() (int64, error) {
	var total int64
	for name := range s.known {
		sz, err := s.calculateRepoSizeLocked(name)
		if err != nil {
			return 0, err
		}
		total += sz
	}
	return total, nil
}

// Close releases all resources by closing all opened child repositories.
func (s *localSuperRepository) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var firstErr error
	for name, repo := range s.repos {
		if closer, ok := repo.(interface{ Close() error }); ok {
			if err := closer.Close(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		delete(s.repos, name)
	}
	return firstErr
}

type repoManagerService struct {
	bboltproto.UnimplementedRepoManagerServiceServer
	super SuperRepository
}

func newRepoManagerService(super SuperRepository) *repoManagerService {
	return &repoManagerService{super: super}
}

func (s *repoManagerService) ListRepos(
	ctx context.Context,
	req *bboltproto.ListReposRequest,
) (*bboltproto.ListReposReply, error) {
	names, err := s.super.List(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list repos: %v", err)
	}
	return &bboltproto.ListReposReply{Names: names}, nil
}

func (s *repoManagerService) CreateRepo(
	ctx context.Context,
	req *bboltproto.CreateRepoRequest,
) (*bboltproto.CreateRepoReply, error) {
	GlobalLogger.Info("repoManagerService remote repository")
	name := strings.TrimSpace(req.GetName())
	if name == "" {
		return nil, status.Errorf(codes.InvalidArgument, "empty repo name")
	}

	if _, err := s.super.Create(ctx, name); err != nil {
		if errors.Is(err, ErrQuotaExceeded) {
			return nil, status.Errorf(codes.ResourceExhausted, "create repo %q: %v", name, err)
		}
		return nil, status.Errorf(codes.Internal, "create repo %q: %v", name, err)
	}
	return &bboltproto.CreateRepoReply{Name: name}, nil
}

func (s *repoManagerService) GetRepoQuota(
	ctx context.Context,
	req *bboltproto.GetRepoQuotaRequest,
) (*bboltproto.GetRepoQuotaResponse, error) {
	q, err := s.super.GetQuota(ctx, req.GetName())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get quota: %v", err)
	}
	return &bboltproto.GetRepoQuotaResponse{
		MaxSizeBytes:    q.MaxSizeBytes,
		ActualSizeBytes: q.ActualSizeBytes,
	}, nil
}

func (s *repoManagerService) SetRepoQuota(
	ctx context.Context,
	req *bboltproto.SetRepoQuotaRequest,
) (*bboltproto.SetRepoQuotaResponse, error) {
	if err := s.super.SetQuota(ctx, req.GetName(), req.GetMaxSizeBytes()); err != nil {
		return nil, status.Errorf(codes.Internal, "set quota: %v", err)
	}
	return &bboltproto.SetRepoQuotaResponse{}, nil
}

func (s *repoManagerService) GetSuperQuota(
	ctx context.Context,
	req *bboltproto.GetSuperQuotaRequest,
) (*bboltproto.GetSuperQuotaResponse, error) {
	q, err := s.super.GetTotalQuota(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get super quota: %v", err)
	}
	return &bboltproto.GetSuperQuotaResponse{
		MaxSizeBytes:    q.MaxSizeBytes,
		ActualSizeBytes: q.ActualSizeBytes,
	}, nil
}

func (s *repoManagerService) SetSuperQuota(
	ctx context.Context,
	req *bboltproto.SetSuperQuotaRequest,
) (*bboltproto.SetSuperQuotaResponse, error) {
	if err := s.super.SetTotalQuota(ctx, req.GetMaxSizeBytes()); err != nil {
		return nil, status.Errorf(codes.Internal, "set super quota: %v", err)
	}
	return &bboltproto.SetSuperQuotaResponse{}, nil
}

func (s *repoManagerService) GetMaxRepoCount(
	ctx context.Context,
	req *bboltproto.GetMaxRepoCountRequest,
) (*bboltproto.GetMaxRepoCountResponse, error) {
	return &bboltproto.GetMaxRepoCountResponse{Count: int32(s.super.GetMaxRepositoryCount(ctx))}, nil
}

func (s *repoManagerService) SetMaxRepoCount(
	ctx context.Context,
	req *bboltproto.SetMaxRepoCountRequest,
) (*bboltproto.SetMaxRepoCountResponse, error) {
	if err := s.super.SetMaxRepositoryCount(ctx, int(req.GetCount())); err != nil {
		return nil, status.Errorf(codes.Internal, "set max repo count: %v", err)
	}
	return &bboltproto.SetMaxRepoCountResponse{}, nil
}

func (s *repoManagerService) DeleteRepo(
	ctx context.Context,
	req *bboltproto.DeleteRepoRequest,
) (*bboltproto.DeleteRepoReply, error) {
	name := strings.TrimSpace(req.GetName())
	if name == "" {
		return nil, status.Errorf(codes.InvalidArgument, "empty repo name")
	}

	if err := s.super.Delete(ctx, name); err != nil {
		return nil, status.Errorf(codes.Internal, "delete repo %q: %v", name, err)
	}
	return &bboltproto.DeleteRepoReply{}, nil
}

type SuperRepositoryServer struct {
	*grpc.Server
	super SuperRepository
}

// NewSuperServer creates a gRPC server that exposes a SuperRepository over the network.
// It initializes a local SuperRepository at the configured directory and registers
// all necessary gRPC services for remote repository access.
//
// The function performs the following:
//   - Creates/opens a local SuperRepository at c.RepoDir
//   - Registers the Bleve index handler for all repositories
//   - Sets up gRPC services: Dataset, Ref, Commit, Index, and RepoManager
//   - Enables gRPC reflection for client discovery
//
// The returned SuperRepositoryServer wraps the gRPC server and provides methods
// for graceful shutdown. Use GracefulStopAndClose() to properly stop the server
// and release all resources.
//
// Returns an error if the SuperRepository cannot be created or if service
// registration fails.
func NewSuperServer(c *RepositoryServerConfig, opts ...grpc.ServerOption) (*SuperRepositoryServer, error) {
	// for now, all repository in the superRepository will use the same bleve deriveInfo
	// Enable per-repo authorization for SuperRepositories, but only when a real
	// OIDC issuer is configured. The test://issuer bypasses RBAC, so per-repo
	// checks cannot be enforced there.
	c.PerRepoAuth = c.Issuer != "" && c.Issuer != "test://issuer"
	super, err := NewLocalSuperRepository(c.RepoDir)
	if err != nil {
		return nil, err
	}
	err = super.RegisterIndexHandler(c.DeriveInfo)
	if err != nil {
		return nil, err
	}

	r, err := super.Get(context.TODO(), "default")
	if err != nil {
		return nil, err
	}

	// if r.Bleve() == nil {
	// 	r.RegisterIndexHandler(super.deriveInfo)
	// }

	s := newServerWithConfig(c)

	dsService := datasetServiceServer{r: r, sr: super, clientID: c.ClientID, perRepoAuth: c.PerRepoAuth, TimeNow: time.Now}
	bboltproto.RegisterDatasetServiceServer(s, &dsService)
	GlobalLogger.Info("datasetService has been registered")

	refService := refServiceServer{R: r, sr: super}
	bboltproto.RegisterRefServiceServer(s, &refService)
	GlobalLogger.Info("refService has been registered")

	commitService := commitServiceServer{r: r, sr: super}
	bboltproto.RegisterCommitServiceServer(s, &commitService)
	GlobalLogger.Info("commitService has been registered")

	bleveproto.RegisterIndexServiceServer(s, initIndexServiceServer(r, super, c.ClientID, c.PerRepoAuth, repositoryMethodRoles))
	GlobalLogger.Info("IndexService has been registered")

	bboltproto.RegisterRepoManagerServiceServer(s, newRepoManagerService(super))
	GlobalLogger.Info("RepoManagerService has been registered")

	reflection.Register(s)

	srv := &SuperRepositoryServer{
		Server: s,
		super:  super,
	}

	return srv, nil
}

// GracefulStopAndClose gracefully stops the gRPC server and closes the repository.
func (s SuperRepositoryServer) GracefulStopAndClose() error {
	GlobalLogger.Info("gRPC server call GracefulStopAndClose")
	s.GracefulStop()

	var err error
	for _, repo := range s.super.(*localSuperRepository).repos {
		err = repo.Close()
	}
	return err
}

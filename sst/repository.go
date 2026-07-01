// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/blevesearch/bleve/v2/search/query"
	"github.com/google/uuid"
	"github.com/semanticstep/sst-core/sstauth"
	"go.etcd.io/bbolt"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/oauth"
)

// Repository is a place to persistently store many related Datasets, consisting of their default NamedGraph and other imported NamedGraphs.
// Depending of the type of Repository the following features are available:
// * logging capability, representing this history of write actions by data and time
// * revision history of each Dataset using functionality similar to GIT
// * query capability on the content of Datasets/NamedGraphs using Bleve

// The following Repository types are available:
// * remote Repository with log, query and Dataset revision history, using BBolt and Bleve
// * local Repository with log, query and Dataset revision history, using BBolt and Bleve
// * local Repository with log and query, but without Dataset revision, using BBolt and Bleve
// * local Repository without log, query and Dataset revision, using of a flat directory of SST files

// All Repository methods have a context parameter to specify things like timeout e.g. for remote Repository access.
// Context parameter is also used for user identification and authentication.
// User identification is done by email address stored in the context. TBD: To be clarified.
// SST identifies Datasets by a UUID that is derived from the Dataset IRI.
type Repository interface {
	// URL returns the specific location where this repository is stored.
	// For a remote main repository (not an item in a SuperRepository) the returned URL has NO FRAGMENT.
	// For a remote repository that is an item in a SuperRepository the returned URL has a Fragment that is the name/id of the repository.
	// For a local repository the returned URL is URL for the local path (normal/default or item in a super repository).
	URL() string

	// SuperRepository() either returns the SuperRepository this Repository belongs to or otherwise return a nil value.
	SuperRepository() SuperRepository

	// OpenStage(DefaultTriplexMode) will create an empty stage to work on persistent data in this SST Repository to which it is linked to.
	// Linking means that data can be loaded from the SST Repository to the Stage and written back to it.
	// As a result, Datasets from this Repository can be loaded, modified, committed or created anew to this SST Repository.
	// TODO: Several Stages for the same Repository can be opened.
	OpenStage(mode TriplexMode) Stage

	// ForDatasets calls given callback function c for each Dataset in the Repository.
	// In case the callback function c returns an error, the loop terminates and this method
	// is returning this error.
	ForDatasets(ctx context.Context, c func(ds Dataset) error) error

	// ForDocuments calls given callback function c for each DocumentInfo in the Repository.
	// In case the callback function c returns an error, the loop terminates and this method
	// is returning this error.
	ForDocuments(ctx context.Context, c func(d DocumentInfo) error) error

	// Datasets retrieves all dataset IRIs from the SST Repository.
	// If there is no Dataset stored in the Repository, an empty slice will be returned.
	Datasets(ctx context.Context) ([]IRI, error)

	// Dataset returns the existing Dataset from the Repository.
	Dataset(ctx context.Context, iri IRI) (Dataset, error)

	// CommitDetails returns commit metadata for the given list of commit hashes.
	CommitDetails(ctx context.Context, hashes []Hash) ([]*CommitDetails, error)

	// CheckoutCommit materializes a Stage at the given repository commit.
	// The Stage contains the Named Graphs recorded in CommitDetails.NamedGraphRevisions
	// for that commit, each at the correct NamedGraphRevision, with checkout lineage metadata set.
	CheckoutCommit(ctx context.Context, commitID Hash, mode TriplexMode) (Stage, error)

	// ----- Index management -----

	// A new created sst.Repository has initially no indexMapping, meaning there is no query functionality.
	// RegisterIndexHandler checks if the provided DerivePackageVersion is same with the DerivePackageVersion in the Repository.
	// If they are different,  the complete recreation of the bleve index will be performed.
	// Application might use the default index mapping provided by defaultderive.DeriveInfo()
	// or create their own specific index mapping.
	// IndexMapping can be empty SSTDeriveInfo struct to remove an existing bleve index.
	// RegisterIndexHandler is only available for local Repositories.
	// Remote repository has predefined index handler which can modified/recreated only on the installation/deployment level.
	// When the Repository is a member of a SuperRepository, this will return ErrNotAvailable.
	RegisterIndexHandler(*SSTDeriveInfo) error

	// Bleve() returns the bleve index if one is registered.
	// A nil value is returned if no one is registered.
	// For Remote Repository, it returns the bleve client. So there is no need to add context parameter.
	Bleve() bleve.Index

	// RebuildBleveIndex closes the current Bleve index, removes the index directory,
	// and rebuilds it from scratch.
	//
	// Supported by:
	//   - localFullRepository: full rebuild with revision history preserved.
	//   - localBasicRepository: full rebuild without revision history.
	//
	// Not supported by:
	//   - remoteRepository: returns ErrNotSupported (rebuild is a server-side operation).
	//   - localFlatRepository: returns ErrNotAvailable (no Bleve index).
	//   - localFlatFileSystemRepository: returns ErrNotAvailable (no Bleve index).
	RebuildBleveIndex() error

	// Close closes the Repository
	Close() error

	// Info returns basic information about the repository content and history.
	// If the the branchName is an empty string, then NumberOfDatasetsInBranch contains those Datasets that are in any branch.
	// Otherwise, NumberOfDatasetsInBranch lists only those Datasets that are in the specified branch.
	Info(ctx context.Context, branchName string) (RepositoryInfo, error)

	// for internal Dataset.Commit handling
	commitNewVersion(ctx context.Context, st *stage, message string, branch string) (Hash, []uuid.UUID, error)

	// Log returns a list of log entries.
	//
	// Parameters:
	// - `start`: the starting logKey (inclusive). If nil or 0, starts from the latest logKey.
	// - `end`:
	//     - If negative, returns at most `-end` entries.
	//     - If positive, defines the exclusive lower bound (entries with logKey > end will be returned).
	//       That is, entries with logKey ≤ end will be excluded.
	//
	// Behavior Examples:
	//   start=nil, end=-100      → return the latest 100 entries
	//   start=123456, end=-50    → return 50 entries from logKey 123456 down to 123407 (inclusive)
	//   start=123, end=100       → return entries from 123 down to 101 (inclusive); excludes 100
	//   start=nil, end=nil       → return all entries from latest to earliest
	Log(ctx context.Context, start, end *int) ([]RepositoryLogEntry, error)

	// LogMessage writes a message into the log together with timestamp and author.
	// LogMessage(ctx context.Context, message string) error

	// NamedGraphDiff(namedGraphRevision1, namedGraphRevision2 Hash, w io.Writer, writeDiffTriple bool) ([]DiffTriple, error)

	// ExtractSstFile extracts a particular revision of a NamedGraph into a io.Writer that will create a SST-file.
	ExtractSstFile(ctx context.Context, namedGraphRevision Hash, w io.Writer) error

	// Only used in remoteRepository and LocalFullRepository.
	// For RemoteRepository, it can sync from a LocalFullRepository to the method receiver.
	// For LocalFullRepository, it can sync from a RemoteRepository to the method receiver.
	// Options can be used to specify which datasets and branches to sync:
	// - If no options are provided, syncs all datasets and all branches.
	// - Use WithDatasetIRIs() to specify which datasets to sync (including their imported dependencies).
	// - Use WithBranch() to specify which branch to sync (use "*" or empty string for all branches).
	SyncFrom(ctx context.Context, from Repository, options ...SyncOption) error

	// DocumentSet stores the given document in the repository and returns its hash.
	// The document is read from source and associated with the provided MIME type.
	DocumentSet(ctx context.Context, MIMEType string, source *bufio.Reader) (Hash, error)

	// Document retrieves the document with the given hash and writes it to target.
	// It returns the associated document info.
	// If target is nil, then only the DocumentInfo is returned
	Document(ctx context.Context, hash Hash, target io.Writer) (*DocumentInfo, error)

	// Documents returns a list of all document metadata stored in the repository.
	Documents(ctx context.Context) ([]DocumentInfo, error)

	// DocumentDelete removes the document content and its DocumentInfo from the repository using the given hash.
	DocumentDelete(ctx context.Context, hash Hash) error
}

type (
	// SSTDeriveInfo describes which bleve package the Repository is using.
	// [Repository.RegisterIndexHandler] needs this struct.
	// Repository method [Repository.IndexMapping] returns the DerivePackageName and DerivePackageVersion.
	SSTDeriveInfo struct {
		DerivePackageName    string                                                                                                                      `json:"DerivePackageName"`
		DerivePackageVersion string                                                                                                                      `json:"DerivePackageVersion"`
		DefaultIndexMapping  func() *mapping.IndexMappingImpl                                                                                            `json:"-"`
		DeriveDocuments      func(r Repository, g NamedGraph) (id string, document map[string]interface{}, preConditionQueries []query.Query, err error) `json:"-"`
	}
)

func (p SSTDeriveInfo) isEmpty() bool {
	return p.DerivePackageName == "" && p.DerivePackageVersion == ""
}

var (
	ErrTerminalRepositoryOptionHasHasWrongPosition = errors.New("terminal repository option has has wrong position")
	ErrTerminalRepositoryOptionMissing             = errors.New("terminal repository option missing")
	ErrUnsupported                                 = errors.New("unsupported")
	ErrRepositoryNotFound                          = errors.New("repository not found")
	ErrDatasetNotFound                             = errors.New("dataset not found")
	ErrDatasetAlreadyExists                        = errors.New("dataset already exists")
	ErrNotAvailable                                = errors.New("not available")
	ErrNotSupported                                = errors.New("not supported")
	errRepositoryAlreadyExists                     = errors.New("sst repository already exists")
	ErrRepositoryDoesNotExist                      = errors.New("sst repository does not exist")
	ErrNotALocalFlatRepository                     = errors.New("not a LocalFlat Repository")
	ErrNotALocalFullRepository                     = errors.New("not a LocalFull Repository")
	ErrNotALocalBasicRepository                    = errors.New("not a LocalBasic Repository")
	ErrUnrecognizedOption                          = errors.New("unrecognized option")
	ErrBranchNotFound                              = errors.New("branch not found")
	ErrEmptyCommitMessage                          = errors.New("empty commit message")
	ErrQuotaExceeded                               = errors.New("repository quota exceeded")
)

// used for the local repositories, not for the remote one.
type repoConfig struct {
	timeNow       func() time.Time
	repositoryDir string

	preCommitCondition     func(stage Stage) (postCommitNotifier, error)
	postCommitNotification func(stage Stage, commitID Hash)

	// bleveIndexUpdater decouples Bleve index maintenance from BBolt writes.
	// A nil value means no Bleve updates are performed.
	bleveIndexUpdater bleveIndexUpdaterI

	// application should register this by calling RegisterIndexHandler
	deriveInfo *SSTDeriveInfo
}

var deriveInfoPlaceholder = &SSTDeriveInfo{
	DerivePackageName:    "",
	DerivePackageVersion: "",
	DefaultIndexMapping:  nil,
	DeriveDocuments:      nil,
}

// OpenRemoteRepository initializes and opens a remote SST repository.
//
// Parameters:
//   - ctx: The context for managing request deadlines and cancellation signals, also contains the [AuthProvider].
//   - targetURL: The URI of the remote repository server.
//   - tlsOption: The gRPC dial option for TLS configuration.
//
// Returns:
//   - Repository: The opened remote repository instance.
//   - error: An error if the repository could not be opened.
//
// The function performs the following steps:
//  1. Sets up the gRPC connection options, including TLS configuration.
//  2. Dials the remote repository server using the provided URI and connection options.
//  3. Retrieves the OAuth2 token from the context's auth provider and adds it to the gRPC call options.
//  4. Attempts to get the repository's index mapping to verify the connection.
//  5. Returns the repository instance if successful, or an error if any step fails.
func OpenRemoteRepository(ctx context.Context, URL string, tlsOption grpc.DialOption) (Repository, error) {
	// DialOption configures how we set up the connection.
	var dialOptions []grpc.DialOption
	dialOptions = append(dialOptions, tlsOption)

	var r Repository
	var err error

	// if there is a fragment in the URL, it means it is a repository in a super repository
	addr, repo, found := strings.Cut(URL, "#")

	if found {
		GlobalLogger.Debug("Opening remote repository as part of a super repository", zap.String("url", URL))
		sr, err := OpenRemoteSuperRepository(ctx, addr, tlsOption)
		if err != nil {
			return nil, err
		}
		r, err = sr.Get(ctx, repo)
		if err != nil {
			return nil, err
		}
	} else {
		GlobalLogger.Debug("Opening remote repository", zap.String("url", URL))
		r, err = dialRepository(URL, dialOptions)
		if err != nil {
			return nil, err
		}
	}

	var opts []grpc.CallOption
	authProvider := sstauth.AuthProviderFromContext(ctx)
	if p, ok := authProvider.(sstauth.Provider); ok {
		token, err := p.Oauth2Token()
		if err != nil {
			return nil, err
		}
		opts = append(opts, grpc.PerRPCCredentials(oauth.NewOauthAccess(token)))
	}

	// try to get Remote Repository RepositoryInfo
	// this is checking if this is an effective connection
	repoInfo, err := r.Info(ctx, "")
	if err != nil {
		return nil, err
	}
	GlobalLogger.Debug(
		"remote repository opened",
		zap.String("bleveName", repoInfo.BleveName),
		zap.String("bleveVersion", repoInfo.BleveVersion),
	)

	return r, nil
}

// CreateLocalRepository creates a new local repository at the specified directory.
// It returns a Repository or an error if any issues occur during the process.
//
// Parameters:
// - repoDir: The directory path where the repository will be created.
// - email: The email address of the user creating the repository.
// - name: The name of the user creating the repository.
// - revisionHistory: A boolean indicating whether to enable revision history for the repository.
//
// Returns:
// - Repository: An interface representing the created repository.
// - error: An error if the repository cannot be created or if there are issues during the creation process.
//
// The function performs the following steps:
// 1. Checks if the specified directory already exists. If it does, it returns errRepositoryAlreadyExists.
// 2. If there are other errors (e.g., permission issues), it returns the error.
// 3. If the directory does not exist, it creates the directory with appropriate permissions.
// 4. Constructs the path to the bbolt database file.
// 5. Creates and opens the bbolt database with specific options (e.g., no freelist sync, using a map-type freelist).
// 6. If revisionHistory is true, it creates a LocalFullRepository with history supporting.
// 7. If revisionHistory is false, it creates a local repository without history supporting.
//
// Example usage:
//
//	repo, err := CreateLocalRepository("/path/to/repo", "default@semanticstep.net", "default", true)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	// Use the created repository (repo)...
func CreateLocalRepository(repoDir string, email string, name string, revisionHistory bool) (Repository, error) {
	_, err := os.Stat(repoDir)
	if err == nil {
		// err is nil means this directory already exists
		return nil, errRepositoryAlreadyExists
	} else {
		// if err is ErrNotExist, creates a new directory
		if os.IsNotExist(err) {
			err = os.MkdirAll(repoDir, 0o755)
			if err != nil {
				return nil, err
			}
		} else { // other errors, return it
			return nil, err
		}
	}
	abs, err := filepath.Abs(repoDir)
	if err != nil {
		return nil, err
	}

	// URL must be a forward slash
	path := filepath.ToSlash(abs)

	// Windows: URL path needs to be in the form of /C:/...
	if runtime.GOOS == "windows" {
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
	}

	bboltPath := filepath.Join(repoDir, bboltName)
	createdBbolt, err := bbolt.Open(bboltPath, 0o666, &bbolt.Options{
		Timeout:        0,
		NoFreelistSync: true,
		FreelistType:   bbolt.FreelistMapType,
	})
	if err != nil {
		return nil, err
	}

	// if revisionHistory is true, create a LocalFullRepository
	if revisionHistory {
		repo := &localFullRepository{
			url:         &url.URL{Scheme: "file", Path: path},
			config:      repoConfig{repositoryDir: repoDir, timeNow: time.Now, deriveInfo: deriveInfoPlaceholder},
			authInfo:    &sstauth.SstUserInfo{Email: email},
			db:          createdBbolt,
			accessRight: sstauth.AccessMode_ReadWrite,
		}

		// Write initial log entry
		if err := writeInitialLogEntry(createdBbolt, name, email); err != nil {
			_ = createdBbolt.Close()
			return nil, fmt.Errorf("failed to write initial log entry: %w", err)
		}

		repo.state.Store(int32(stateOpen))
		return repo, nil
	} else { // create a localBasicRepository
		localBasicRepo := &localBasicRepository{
			url:                  &url.URL{Scheme: "file", Path: path},
			config:               repoConfig{repositoryDir: repoDir, timeNow: time.Now, deriveInfo: deriveInfoPlaceholder},
			repositoryBucketName: rootRepositoryBucketName,
			db:                   createdBbolt,
			accessRight:          sstauth.AccessMode_ReadWrite,
		}
		localBasicRepo.state.Store(int32(stateOpen))
		// Initialize the basic repository
		err = localBasicRepo.update(func(rFS fs.FS) error { return nil })
		if err != nil {
			_ = createdBbolt.Close()
			return nil, err
		}

		// Write initial log entry
		if err := writeInitialLogEntry(createdBbolt, name, email); err != nil {
			_ = createdBbolt.Close()
			return nil, fmt.Errorf("failed to write initial log entry: %w", err)
		}

		return localBasicRepo, nil
	}
}

// writeInitialLogEntry creates a sub-bucket at log/0 and writes "timestamp" & "message" & "author" field into it.
func writeInitialLogEntry(db *bbolt.DB, name, email string) error {
	timestamp := time.Now().UTC().Format(time.RFC3339)
	author := fmt.Sprintf("%s <%s>", name, email)

	return db.Update(func(tx *bbolt.Tx) error {
		// Create or get top-level log bucket
		logBucket, err := tx.CreateBucketIfNotExists([]byte("log"))
		if err != nil {
			return fmt.Errorf("failed to create log bucket: %w", err)
		}

		// log key = 0 → encoded to 8-byte big endian
		logKey := make([]byte, 8)
		binary.BigEndian.PutUint64(logKey, 0)

		// Create sub-bucket under key = 0
		entryBucket, err := logBucket.CreateBucket(logKey)
		if err != nil {
			return fmt.Errorf("failed to create sub-bucket for log entry: %w", err)
		}

		// Write "timestamp" field inside sub-bucket
		if err := entryBucket.Put([]byte("type"), []byte("init")); err != nil {
			return fmt.Errorf("failed to write message: %w", err)
		}
		if err := entryBucket.Put([]byte("timestamp"), []byte(timestamp)); err != nil {
			return fmt.Errorf("failed to write timestamp: %w", err)
		}
		if err := entryBucket.Put([]byte("message"), []byte("repository created")); err != nil {
			return fmt.Errorf("failed to write message: %w", err)
		}
		if err := entryBucket.Put([]byte("author"), []byte(author)); err != nil {
			return fmt.Errorf("failed to write author: %w", err)
		}

		return nil
	})
}

// OpenLocalRepository opens an existing local repository located at the specified directory.
// It returns a Repository interface and an error if any issues occur during the process.
//
// Parameters:
// - repoDir: The directory path where the repository is located.
// - email: The email address of the user accessing the repository.
// - name: The name of the user accessing the repository.
//
// Returns:
// - Repository: An interface representing the opened repository.
// - error: An error if the repository does not exist or if there are issues opening it.
//
// The function performs the following steps:
// 1. Checks if the specified directory exists. If it does not exist, it returns ErrRepositoryDoesNotExist.
// 2. If there are other errors (e.g., permission issues), returns it.
// 3. If the directory exists, it constructs the path to the bbolt database file.
// 4. Opens the bbolt database with specific options (e.g., no freelist sync, using a map-type freelist).
// 5. If there is an error opening the bbolt database, returns it.
//
// Example usage:
//
//	repo, err := OpenLocalRepository("/path/to/repo", "default@semanticstep.net", "default")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	// Use the opened repository (repo)...
func OpenLocalRepository(repoDir string, email string, name string) (Repository, error) {
	// Check if the directory exists
	if _, err := os.Stat(repoDir); os.IsNotExist(err) {
		// Directory does not exist, return error
		return nil, ErrRepositoryDoesNotExist
	} else if err != nil {
		// Other errors (e.g., permission issues)
		return nil, err
	} else {
		// Directory exists, continue
	}

	abs, err := filepath.Abs(repoDir)
	if err != nil {
		return nil, err
	}

	// URL must be a forward slash
	path := filepath.ToSlash(abs)

	// Windows: URL path needs to be in the form of /C:/...
	if runtime.GOOS == "windows" {
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
	}

	bboltPath := filepath.Join(repoDir, bboltName)
	openedBBolt, err := bbolt.Open(bboltPath, 0o666, &bbolt.Options{
		Timeout:        0,
		NoFreelistSync: true,
		FreelistType:   bbolt.FreelistMapType,
	})
	if err != nil {
		return nil, err
	}

	var isLocalBasic bool
	err = openedBBolt.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(rootRepositoryBucketName)
		if b != nil {
			isLocalBasic = true
		}
		return nil
	})
	if err != nil {
		log.Fatalf("Error checking bucket existence: %v", err)
	}

	var returnRepo Repository
	if isLocalBasic {
		returnRepo = &localBasicRepository{
			url:                  &url.URL{Scheme: "file", Path: path},
			config:               repoConfig{repositoryDir: repoDir, timeNow: time.Now, deriveInfo: deriveInfoPlaceholder},
			repositoryBucketName: rootRepositoryBucketName,
			db:                   openedBBolt,
			accessRight:          accessModeFromDir(repoDir),
		}
		returnRepo.(*localBasicRepository).state.Store(int32(stateOpen))
	} else {
		returnRepo = &localFullRepository{
			url:         &url.URL{Scheme: "file", Path: path},
			config:      repoConfig{repositoryDir: repoDir, timeNow: time.Now, deriveInfo: deriveInfoPlaceholder},
			authInfo:    &sstauth.SstUserInfo{Email: email},
			db:          openedBBolt,
			accessRight: accessModeFromDir(repoDir),
		}
		returnRepo.(*localFullRepository).state.Store(int32(stateOpen))
	}

	return returnRepo, nil
}

// CreateLocalFlatRepository creates a new LocalFlatRepository at the specified path.
// If the directory already exists, it returns an error indicating that the repository already exists.
// If the directory does not exist, it creates a new directory with the specified permissions.
//
// Parameters:
//   - repoDir: The path to the directory where the repository should be created.
//
// Returns:
//   - Repository: The created repository object.
//   - error: An error if the repository could not be created or if the directory already exists.
//
// Possible errors:
//   - errRepositoryAlreadyExists: The directory already exists.
//   - errDirectoryExpected: The path exists but is not a directory.
//   - Other errors related to directory creation and path resolution.
func CreateLocalFlatRepository(repoDir string) (Repository, error) {
	rc := repoConfig{
		repositoryDir: repoDir,
		deriveInfo:    deriveInfoPlaceholder,
	}

	_, err := os.Stat(rc.repositoryDir)
	// err is nil means this directory already exists
	if err == nil {
		return nil, errRepositoryAlreadyExists
	} else {
		// create a new directory
		if os.IsNotExist(err) {
			err = os.MkdirAll(rc.repositoryDir, 0o755)
			if err != nil {
				return nil, err
			}
		}
	}
	dir, err := filepath.Abs(rc.repositoryDir)
	if err != nil {
		return nil, err
	}

	// URL must be a forward slash
	path := filepath.ToSlash(dir)

	// Windows: URL path needs to be in the form of /C:/...
	if runtime.GOOS == "windows" {
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
	}

	dirStat, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !dirStat.IsDir() {
		return nil, errDirectoryExpected
	}

	rc.repositoryDir = dir

	return &localFlatRepository{
		url:         &url.URL{Scheme: "file", Path: path},
		config:      rc,
		accessRight: sstauth.AccessMode_ReadWrite,
	}, err
}

// LocalFlatRepository is a kind of SST Repository where NamedGraphs are written as individual SST Binary files in the local file system.
// As each SST Binary file is written independently no transaction functionality is provided.
// No change history is recorded for this Repository kind.
// The names of the SST Binary files are named by their UUID and suffixed by ".sst"
//
//	Eg. 3ffe6728-9a14-4ae8-8db7-5b4ae7984ab9.sst
//
// If a NamedGraph is identified by arbitrary IRI that is not urn-uuid then UUID is derived from the IRI as SHA1 (Version 5) UUID.
// Note: The full IRI of a NamedGraph is stored within the SST Binary file.
//
// OpenLocalFlatRepository opens a LocalFlatRepository.
// It takes the path to the repository directory as an argument and returns a Repository instance
// or an error if the directory does not exist or is not a directory.
//
// Parameters:
//   - repoDir: The path to the repository directory.
//
// Returns:
//   - Repository: The repository instance if the directory exists and is valid.
//   - error: An error if the directory does not exist, is not a directory, or if there is any other issue.
func OpenLocalFlatRepository(repoDir string) (Repository, error) {
	rc := repoConfig{
		repositoryDir: repoDir,
		deriveInfo:    deriveInfoPlaceholder,
	}

	_, err := os.Stat(rc.repositoryDir)
	if err != nil {
		if os.IsNotExist(err) {
			// repository directory does not exist
			return nil, ErrRepositoryDoesNotExist
		}
	}
	dir, err := filepath.Abs(rc.repositoryDir)
	if err != nil {
		return nil, err
	}

	// URL must be a forward slash
	path := filepath.ToSlash(dir)

	// Windows: URL path needs to be in the form of /C:/...
	if runtime.GOOS == "windows" {
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
	}

	dirStat, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}

	if !dirStat.IsDir() {
		return nil, errDirectoryExpected
	}

	rc.repositoryDir = dir

	return &localFlatRepository{
		url:         &url.URL{Scheme: "file", Path: path},
		config:      rc,
		accessRight: accessModeFromDir(dir),
	}, err
}

func OpenLocalFlatFileSystemRepository(dictFS fs.FS) (Repository, error) {
	// check file system root
	_, err := dictFS.Open(".")
	if err != nil {
		return nil, err
	}

	return &localFlatFileSystemRepository{fs: dictFS}, nil
}

// RepositoryInfo represents the statistics of a repository.
type RepositoryInfo struct {
	URL                         string             `json:"url"`
	AccessRight                 sstauth.AccessMode `json:"AccessRight"`
	MasterDBSize                int64              `json:"masterDBSize"`  // BboltSize
	DerivedDBSize               int64              `json:"derivedDBSize"` // BleveSize
	DocumentDBSize              int64              `json:"documentDBSize"`
	ActualRepositorySize        int64              `json:"actualRepositorySize"`
	MaxRepositorySize           int64              `json:"maxRepositorySize"`
	NumberOfDatasets            int64              `json:"numberOfDatasets"`
	NumberOfDatasetsInBranch    int64              `json:"numberOfDatasetsInBranch"`
	NumberOfDatasetRevisions    int64              `json:"numberOfDatasetRevisions"`
	NumberOfNamedGraphRevisions int64              `json:"numberOfNamedGraphRevisions"`
	NumberOfCommits             int64              `json:"numberOfCommits"`
	NumberOfRepositoryLogs      int64              `json:"NumberOfRepositoryLogs"`
	NumberOfDocuments           int64              `json:"NumberOfDocuments"`
	IsRemote                    bool               `json:"isRemote"`
	SupportRevisionHistory      bool               `json:"supportRevisionHistory"`
	BleveName                   string             `json:"bleveName"`
	BleveVersion                string             `json:"bleveVersion"`
	VersionHash                 string             `json:"versionHash"`
}

// RepositoryQuota describes the configured size limit and current usage for a repository.
type RepositoryQuota struct {
	MaxSizeBytes    int64 `json:"maxSizeBytes"`
	ActualSizeBytes int64 `json:"actualSizeBytes"`
}

func (r RepositoryInfo) String() string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false) //
	enc.SetIndent("", "  ")  //
	if err := enc.Encode(r); err != nil {
		return fmt.Sprintf(`{"error": "%v"}`, err)
	}
	return buf.String()
}

// RepositoryLogEntry represents a single entry in the repository log.
//
// Fields:
//
//	Common fields (present in all entries):
//	  - "type":      string, required. Defines the entry type. Possible values:
//	                   "commit", "set_branch", "remove_branch",
//	                   "upload_document", "download_document",
//	                   "delete_document", "sync_from".
//
//	type = "commit":
//	  - "author":    string, Author email.
//	  - "branch":    string, Branch name.
//	  - "commit_id": string, Commit hash.
//
//	type = "set_branch":
//	  - "author":     string,  Author email.
//	  - "dataset":    string,  UUID of the dataset.
//	  - "branch":     string,  Branch name.
//	  - "message":    string,  Additional info.
//	  - "ds_revision": string, Dataset revision hash.
//	  - "timestamp":  string, RFC3339 format.

// type = "remove_branch":
//   - "author":     string,  Author email.
//   - "dataset":    string,  UUID of the dataset.
//   - "branch":     string,  Branch name removed.
//   - "message":    string,  Additional info.
//   - "ds_revision": string, Dataset revision hash.
//   - "timestamp":  string, RFC3339 format.
//
// type = "upload_document":
//   - "author":     string, Author email.
//   - "hash":       string, Document SHA256 hash (hex).
//   - "file_name":  string, Original file name.
//   - "mime_type":  string, MIME type.
//   - "message":    string, Description.
//   - "timestamp":  string, RFC3339 format.
//
// type = "download_document":
//   - "author":     string,. Author email.
//   - "hash":       string, Document SHA256 hash (hex).
//   - "mime_type":  string, MIME type.
//   - "message":    string
//   - "timestamp": string, RFC3339 format.
//
// type = "delete_document":
//   - "author":     string, Author email.
//   - "hash":       string, Document SHA256 hash (hex).
//   - "mime_type":  string, MIME type.
//   - "message":    string
//   - "timestamp":  string, RFC3339 format.
//
// type = "sync_from":
//   - "author":         string, Author email.
//   - "timestamp":      string, RFC3339 format.
//   - "from_repo_url":  string, URL of the source repository.
//   - "affected_count": string, Number of newly synced dataset branch/revision pairs.
//   - "dataset_N":      string, Dataset IRI of affected entry N (0-based).
//   - "branch_N":       string, Named branch or "leaf" for affected entry N.
//   - "ds_revision_N":  string, Dataset revision hash for affected entry N.
type RepositoryLogEntry struct {
	LogKey uint64
	Fields map[string]string // timestamp, type, commit_id, message, ...
}

// SyncOptions contains options for synchronizing data between repositories.
type SyncOptions struct {
	// DatasetIDs is a list of dataset UUIDs to sync.
	// If nil or empty, all datasets will be synced.
	DatasetIDs []uuid.UUID

	// BranchName specifies the branch to sync.
	// Empty string "" or "*" means sync all branches.
	// Otherwise, it specifies a particular branch name (e.g., "master").
	BranchName string
}

// SyncOption is a function that modifies SyncOptions.
type SyncOption func(*SyncOptions)

// defaultSyncOptions returns the default sync options (sync all datasets and all branches).
func defaultSyncOptions() SyncOptions {
	return SyncOptions{
		DatasetIDs: nil,
		BranchName: "", // Empty string means all branches
	}
}

// WithDatasetIRIs specifies which datasets (by IRI) to sync.
// If no dataset IRIs are provided, all datasets will be synced.
func WithDatasetIRIs(datasetIRIs ...IRI) SyncOption {
	return func(opts *SyncOptions) {
		if len(datasetIRIs) == 0 {
			return
		}
		ids := make([]uuid.UUID, 0, len(datasetIRIs))
		for _, iri := range datasetIRIs {
			id, err := iriToUUID(iri)
			if err != nil {
				GlobalLogger.Error("WithDatasetIRIs: invalid IRI", zap.String("iri", string(iri)), zap.Error(err))
			}
			ids = append(ids, id)
		}
		opts.DatasetIDs = ids
	}
}

// WithBranch specifies which branch to sync.
// If branchName is empty string "" or "*", all branches will be synced.
// Otherwise, only the specified branch will be synced.
func WithBranch(branchName string) SyncOption {
	return func(opts *SyncOptions) {
		// Normalize "*" to empty string for consistency
		if branchName == "*" {
			opts.BranchName = ""
		} else {
			opts.BranchName = branchName
		}
	}
}

// isAllBranches checks if the branch name indicates all branches should be synced.
// Returns true if branchName is empty string or "*".
func isAllBranches(branchName string) bool {
	return branchName == "" || branchName == "*"
}

// isDirWritable reports whether the directory dir can be written to.
// It probes actual write capability by creating and removing a temporary
// file inside dir. This is more reliable than os.FileMode.Perm(), which
// misses read-only mounts, ACLs, SELinux, disk-full, and does not reflect
// real permissions on Windows.
func isDirWritable(dir string) bool {
	f, err := os.CreateTemp(dir, ".sst-write-test-*")
	if err != nil {
		return false
	}
	_ = f.Close()
	_ = os.Remove(f.Name())
	return true
}

// accessModeFromDir probes dir for actual write capability and returns the
// corresponding access mode. The value is intended to be cached on a
// repository struct so that Info() can return it without further I/O.
func accessModeFromDir(dir string) sstauth.AccessMode {
	if isDirWritable(dir) {
		return sstauth.AccessMode_Admin
	}
	return sstauth.AccessMode_ReadOnly
}

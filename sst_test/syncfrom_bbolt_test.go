// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/semanticstep/sst-core/defaultderive"
	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/sst_test/testutil"
	"github.com/semanticstep/sst-core/sstauth"
	"github.com/semanticstep/sst-core/vocabularies/lci"
	"github.com/semanticstep/sst-core/vocabularies/rdf"
	"github.com/semanticstep/sst-core/vocabularies/rdfs"
	"github.com/semanticstep/sst-core/vocabularies/rep"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.etcd.io/bbolt"
)

// bbolt bucket names used by SST LocalFullRepository.
const (
	bucketNamedGraphRevisions = "ngr"
	bucketDatasetRevisions    = "dsr"
	bucketDatasets            = "ds"
	bucketCommits             = "c"
	bucketDocumentInfo        = "document_info"
	bucketDatasetLog          = "dl"
)

// key prefix bytes used inside SST bbolt buckets.
const (
	prefixRefBranch     = byte(0x00)
	prefixCommitDataset = byte(0x00)
	prefixDSRevDefault  = byte(0x00)
	prefixDSRevImported = byte(0x00)
	prefixDSRevAllNGs   = byte(0x01)
	prefixDSRevCommit   = byte(0x02)
)

// startRepositoryServer starts a RepositoryServer for the given repository directory
// and returns the server together with the gRPC address. The caller is responsible
// for stopping the server with GracefulStopAndClose before opening the local bbolt.db.
func startRepositoryServer(t *testing.T, repoDir string) (*sst.RepositoryServer, string) {
	t.Helper()

	cert, err := testutil.TestServerCert()
	require.NoError(t, err)

	server, err := sst.NewServer(&sst.RepositoryServerConfig{
		RepoDir:    repoDir,
		Issuer:     "test://issuer",
		ServerCert: &cert,
		Verbose:    true,
		DeriveInfo: defaultderive.DeriveInfo(),
	})
	require.NoError(t, err)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	port := lis.Addr().(*net.TCPAddr).Port
	url := fmt.Sprintf("localhost:%d", port)

	go func() {
		assert.NoError(t, server.Serve(lis))
	}()

	return server, url
}

// openBboltReadOnly opens the bbolt.db inside a repository directory in read-only mode.
func openBboltReadOnly(t *testing.T, repoDir string) *bbolt.DB {
	t.Helper()

	dbPath := filepath.Join(repoDir, "bbolt.db")
	db, err := bbolt.Open(dbPath, 0o600, &bbolt.Options{ReadOnly: true})
	require.NoError(t, err)
	return db
}

// countBucketKeys returns the number of top-level keys in a bbolt bucket.
func countBucketKeys(b *bbolt.Bucket) int {
	count := 0
	_ = b.ForEach(func(_, _ []byte) error {
		count++
		return nil
	})
	return count
}

// branchPointerKey returns the key used inside a Datasets sub-bucket for a branch pointer.
func branchPointerKey(branchName string) []byte {
	return append([]byte{prefixRefBranch}, []byte(branchName)...)
}

// commitDatasetRevKey returns the key used inside a Commits sub-bucket for a dataset revision.
func commitDatasetRevKey(dsID uuid.UUID) []byte {
	return append([]byte{prefixCommitDataset}, dsID[:]...)
}

// dsRevImportedDsKey returns the key used inside a DatasetRevisions sub-bucket
// to reference an imported dataset revision.
func dsRevImportedDsKey(dsID uuid.UUID) []byte {
	return append([]byte{prefixDSRevImported}, dsID[:]...)
}

// dsRevAllNGsKey returns the key used inside a DatasetRevisions sub-bucket
// to reference a NamedGraph revision in the flattened all-NGs list.
func dsRevAllNGsKey(ngID uuid.UUID) []byte {
	return append([]byte{prefixDSRevAllNGs}, ngID[:]...)
}

// dsRevDefaultNGKey returns the key for the default NamedGraph revision inside
// a DatasetRevisions sub-bucket.
func dsRevDefaultNGKey() []byte {
	return []byte{prefixDSRevDefault}
}

// dsRevCommitKey returns the key for the creating commit hash inside a
// DatasetRevisions sub-bucket.
func dsRevCommitKey() []byte {
	return []byte{prefixDSRevCommit}
}

// openRemoteRepoWithURL opens a RemoteRepository for the given URL using test credentials.
func openRemoteRepoWithURL(t *testing.T, ctx context.Context, url string) sst.Repository {
	t.Helper()

	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)

	url = "passthrough:///" + url
	ctx = sstauth.ContextWithAuthProvider(ctx, testutil.TestProviderInstance)

	repo, err := sst.OpenRemoteRepository(ctx, url, transportCreds)
	require.NoError(t, err)
	return repo
}

// createLocalRepoWithOneNG creates a LocalFullRepository with a single NamedGraph
// committed to the master branch.
func createLocalRepoWithOneNG(t *testing.T, dir, label string, ngID uuid.UUID) sst.Repository {
	t.Helper()

	_, repo := createLocalRepoWithOneNGAndCommit(t, dir, label, ngID)
	return repo
}

// createLocalRepoWithOneNGAndCommit creates a LocalFullRepository with a single NamedGraph
// committed to the master branch and returns the commit hash together with the repository.
func createLocalRepoWithOneNGAndCommit(t *testing.T, dir, label string, ngID uuid.UUID) (sst.Hash, sst.Repository) {
	t.Helper()

	os.RemoveAll(dir)
	repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
	require.NoError(t, err)

	st := repo.OpenStage(sst.DefaultTriplexMode)
	graph := st.CreateNamedGraph(sst.IRI(ngID.URN()))
	node := graph.CreateIRINode("main", lci.Organization)
	node.AddStatement(rdfs.Label, sst.String(label))
	commitHash, _, err := st.Commit(context.TODO(), "commit "+label, sst.DefaultBranch)
	require.NoError(t, err)

	return commitHash, repo
}

// createLocalRepoWithTwoUnrelatedNGs creates a LocalFullRepository with two
// independent NamedGraphs, each committed separately to the master branch.
func createLocalRepoWithTwoUnrelatedNGs(t *testing.T, dir, label1, label2 string, ngID1, ngID2 uuid.UUID) sst.Repository {
	t.Helper()

	os.RemoveAll(dir)
	repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
	require.NoError(t, err)

	st1 := repo.OpenStage(sst.DefaultTriplexMode)
	graph1 := st1.CreateNamedGraph(sst.IRI(ngID1.URN()))
	node1 := graph1.CreateIRINode("main1", lci.Organization)
	node1.AddStatement(rdfs.Label, sst.String(label1))
	_, _, err = st1.Commit(context.TODO(), "commit ng1", sst.DefaultBranch)
	require.NoError(t, err)

	st2 := repo.OpenStage(sst.DefaultTriplexMode)
	graph2 := st2.CreateNamedGraph(sst.IRI(ngID2.URN()))
	node2 := graph2.CreateIRINode("main2", lci.Organization)
	node2.AddStatement(rdfs.Label, sst.String(label2))
	_, _, err = st2.Commit(context.TODO(), "commit ng2", sst.DefaultBranch)
	require.NoError(t, err)

	return repo
}

// createLocalRepoWithImportedNGs creates a LocalFullRepository where ngID_A imports
// ngID_B via owl:import, and only ngID_A is explicitly committed.
// The returned commit hash is the single commit that references both datasets.
func createLocalRepoWithImportedNGs(t *testing.T, dir, labelA, labelB string, ngID_A, ngID_B uuid.UUID) (sst.Repository, sst.Hash) {
	t.Helper()

	os.RemoveAll(dir)
	repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
	require.NoError(t, err)

	st := repo.OpenStage(sst.DefaultTriplexMode)

	graphB := st.CreateNamedGraph(sst.IRI(ngID_B.URN()))
	nodeB := graphB.CreateIRINode("mainB", lci.Organization)
	nodeB.AddStatement(rdfs.Label, sst.String(labelB))

	graphA := st.CreateNamedGraph(sst.IRI(ngID_A.URN()))
	require.NoError(t, graphA.AddImport(graphB))
	nodeA := graphA.CreateIRINode("mainA", lci.Organization)
	nodeA.AddStatement(rdfs.Label, sst.String(labelA))
	nodeA.AddStatement(rdf.Type, rep.SchematicPort)

	commitHash, _, err := st.Commit(context.TODO(), "commit A with import B", sst.DefaultBranch)
	require.NoError(t, err)

	return repo, commitHash
}

// requireNGRKeyExists asserts that the given NamedGraph revision hash exists as a key
// in the NamedGraphRevisions bucket and that its value is non-empty.
func requireNGRKeyExists(t *testing.T, ngrBucket *bbolt.Bucket, ngRevHash sst.Hash) {
	t.Helper()

	value := ngrBucket.Get(ngRevHash[:])
	require.NotNil(t, value, "NamedGraphRevision %s not found", ngRevHash.String())
	require.Greater(t, len(value), 0, "NamedGraphRevision %s has empty value", ngRevHash.String())
}

// requireDatasetBranchPointer asserts that the given dataset/branch points to the
// expected commit hash inside the Datasets bucket.
func requireDatasetBranchPointer(t *testing.T, dsBucket *bbolt.Bucket, dsID uuid.UUID, branchName string, expectedCommit sst.Hash) {
	t.Helper()

	dsSub := dsBucket.Bucket(dsID[:])
	require.NotNil(t, dsSub, "dataset %s not found in Datasets bucket", dsID)

	actual := dsSub.Get(branchPointerKey(branchName))
	require.Equal(t, expectedCommit[:], actual, "branch pointer mismatch for dataset %s branch %s", dsID, branchName)
}

// getCommitDSRevision reads the dataset revision hash for dsID from the given
// commit sub-bucket.
func getCommitDSRevision(t *testing.T, commitSub *bbolt.Bucket, dsID uuid.UUID) sst.Hash {
	t.Helper()

	value := commitSub.Get(commitDatasetRevKey(dsID))
	require.NotNil(t, value, "dataset revision for %s not found in commit", dsID)
	require.Len(t, value, sha256.Size, "dataset revision value for %s has wrong length", dsID)
	return sst.BytesToHash(value)
}

// getDSRevisionNGHash reads the default NamedGraph revision hash from a
// DatasetRevisions sub-bucket.
func getDSRevisionNGHash(t *testing.T, dsRevSub *bbolt.Bucket) sst.Hash {
	t.Helper()

	value := dsRevSub.Get(dsRevDefaultNGKey())
	require.NotNil(t, value, "default NG hash not found in DatasetRevision")
	require.Len(t, value, sha256.Size, "default NG hash has wrong length")
	return sst.BytesToHash(value)
}

// getDSRevisionCommitHash reads the creating commit hash from a
// DatasetRevisions sub-bucket.
func getDSRevisionCommitHash(t *testing.T, dsRevSub *bbolt.Bucket) sst.Hash {
	t.Helper()

	value := dsRevSub.Get(dsRevCommitKey())
	require.NotNil(t, value, "commit hash not found in DatasetRevision")
	require.Len(t, value, sha256.Size, "commit hash has wrong length")
	return sst.BytesToHash(value)
}

// Test_RemoteToRemote_BBolt_OneNG verifies the bbolt structure after syncing
// a single NamedGraph from a remote repository to another remote repository.
func Test_RemoteToRemote_BBolt_OneNG(t *testing.T) {
	ngID := uuid.MustParse("a1b2c3d4-e5f6-7890-abcd-ef1234567890")

	sourceDir := t.TempDir()
	targetDir := t.TempDir()

	sourceRepo := createLocalRepoWithOneNG(t, sourceDir, "Alpha", ngID)
	sourceRepo.Close()

	os.RemoveAll(targetDir)
	targetRepo, err := sst.CreateLocalRepository(targetDir, "target@test.com", "target", true)
	require.NoError(t, err)
	targetRepo.Close()

	sourceURL := testutil.ServerServe(t, sourceDir)

	targetServer, targetURL := startRepositoryServer(t, targetDir)

	ctx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
	sourceRemote := openRemoteRepoWithURL(t, ctx, sourceURL)
	targetRemote := openRemoteRepoWithURL(t, ctx, targetURL)

	err = targetRemote.SyncFrom(ctx, sourceRemote)
	require.NoError(t, err)

	sourceRemote.Close()
	targetRemote.Close()
	require.NoError(t, targetServer.GracefulStopAndClose())

	db := openBboltReadOnly(t, targetDir)
	defer db.Close()

	err = db.View(func(tx *bbolt.Tx) error {
		ngrBucket := tx.Bucket([]byte(bucketNamedGraphRevisions))
		require.NotNil(t, ngrBucket)
		require.Equal(t, 1, countBucketKeys(ngrBucket), "NamedGraphRevisions bucket should contain exactly 1 entry")

		dsrBucket := tx.Bucket([]byte(bucketDatasetRevisions))
		require.NotNil(t, dsrBucket)
		require.Equal(t, 1, countBucketKeys(dsrBucket), "DatasetRevisions bucket should contain exactly 1 entry")

		dsBucket := tx.Bucket([]byte(bucketDatasets))
		require.NotNil(t, dsBucket)
		require.Equal(t, 1, countBucketKeys(dsBucket), "Datasets bucket should contain exactly 1 entry")

		commitsBucket := tx.Bucket([]byte(bucketCommits))
		require.NotNil(t, commitsBucket)
		require.Equal(t, 1, countBucketKeys(commitsBucket), "Commits bucket should contain exactly 1 entry")

		docInfoBucket := tx.Bucket([]byte(bucketDocumentInfo))
		require.True(t, docInfoBucket == nil || countBucketKeys(docInfoBucket) == 0, "document_info bucket should be empty")

		// Cross-reference check: ds -> commit -> dsRev -> ngRev
		dsSub := dsBucket.Bucket(ngID[:])
		require.NotNil(t, dsSub)
		commitHashBytes := dsSub.Get(branchPointerKey(sst.DefaultBranch))
		require.Len(t, commitHashBytes, sha256.Size)
		commitHash := sst.BytesToHash(commitHashBytes)

		commitSub := commitsBucket.Bucket(commitHash[:])
		require.NotNil(t, commitSub)
		require.Equal(t, "commit Alpha", string(commitSub.Get([]byte("message"))))

		dsRevHash := getCommitDSRevision(t, commitSub, ngID)

		dsRevSub := dsrBucket.Bucket(dsRevHash[:])
		require.NotNil(t, dsRevSub)
		ngRevHash := getDSRevisionNGHash(t, dsRevSub)
		require.Equal(t, commitHash, getDSRevisionCommitHash(t, dsRevSub))

		requireNGRKeyExists(t, ngrBucket, ngRevHash)

		return nil
	})
	require.NoError(t, err)
}

// Test_RemoteToRemote_BBolt_TwoUnrelatedNGs verifies the bbolt structure after syncing
// two independent NamedGraphs from a remote repository to another remote repository.
func Test_RemoteToRemote_BBolt_TwoUnrelatedNGs(t *testing.T) {
	ngID1 := uuid.MustParse("b2c3d4e5-f6a7-8901-bcde-f23456789012")
	ngID2 := uuid.MustParse("c3d4e5f6-a7b8-9012-cdef-345678901234")

	sourceDir := t.TempDir()
	targetDir := t.TempDir()

	sourceRepo := createLocalRepoWithTwoUnrelatedNGs(t, sourceDir, "Alpha", "Beta", ngID1, ngID2)
	sourceRepo.Close()

	os.RemoveAll(targetDir)
	targetRepo, err := sst.CreateLocalRepository(targetDir, "target@test.com", "target", true)
	require.NoError(t, err)
	targetRepo.Close()

	sourceURL := testutil.ServerServe(t, sourceDir)

	targetServer, targetURL := startRepositoryServer(t, targetDir)

	ctx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
	sourceRemote := openRemoteRepoWithURL(t, ctx, sourceURL)
	targetRemote := openRemoteRepoWithURL(t, ctx, targetURL)

	err = targetRemote.SyncFrom(ctx, sourceRemote)
	require.NoError(t, err)

	sourceRemote.Close()
	targetRemote.Close()
	require.NoError(t, targetServer.GracefulStopAndClose())

	db := openBboltReadOnly(t, targetDir)
	defer db.Close()

	err = db.View(func(tx *bbolt.Tx) error {
		ngrBucket := tx.Bucket([]byte(bucketNamedGraphRevisions))
		require.Equal(t, 2, countBucketKeys(ngrBucket))

		dsrBucket := tx.Bucket([]byte(bucketDatasetRevisions))
		require.Equal(t, 2, countBucketKeys(dsrBucket))

		dsBucket := tx.Bucket([]byte(bucketDatasets))
		require.Equal(t, 2, countBucketKeys(dsBucket))

		commitsBucket := tx.Bucket([]byte(bucketCommits))
		require.Equal(t, 2, countBucketKeys(commitsBucket))

		// Cross-reference check for ngID1.
		dsSub1 := dsBucket.Bucket(ngID1[:])
		require.NotNil(t, dsSub1)
		commitHash1 := sst.BytesToHash(dsSub1.Get(branchPointerKey(sst.DefaultBranch)))
		require.Len(t, commitHash1[:], sha256.Size)

		commitSub1 := commitsBucket.Bucket(commitHash1[:])
		require.NotNil(t, commitSub1)
		require.Equal(t, "commit ng1", string(commitSub1.Get([]byte("message"))))

		dsRevHash1 := getCommitDSRevision(t, commitSub1, ngID1)
		dsRevSub1 := dsrBucket.Bucket(dsRevHash1[:])
		require.NotNil(t, dsRevSub1)
		ngRevHash1 := getDSRevisionNGHash(t, dsRevSub1)
		require.Equal(t, commitHash1, getDSRevisionCommitHash(t, dsRevSub1))
		requireNGRKeyExists(t, ngrBucket, ngRevHash1)

		// dsRev1 should not reference ngID2 because the datasets are unrelated.
		require.Nil(t, dsRevSub1.Get(dsRevImportedDsKey(ngID2)))
		require.Nil(t, dsRevSub1.Get(dsRevAllNGsKey(ngID2)))

		// Cross-reference check for ngID2.
		dsSub2 := dsBucket.Bucket(ngID2[:])
		require.NotNil(t, dsSub2)
		commitHash2 := sst.BytesToHash(dsSub2.Get(branchPointerKey(sst.DefaultBranch)))
		require.Len(t, commitHash2[:], sha256.Size)

		commitSub2 := commitsBucket.Bucket(commitHash2[:])
		require.NotNil(t, commitSub2)
		require.Equal(t, "commit ng2", string(commitSub2.Get([]byte("message"))))

		dsRevHash2 := getCommitDSRevision(t, commitSub2, ngID2)
		dsRevSub2 := dsrBucket.Bucket(dsRevHash2[:])
		require.NotNil(t, dsRevSub2)
		ngRevHash2 := getDSRevisionNGHash(t, dsRevSub2)
		require.Equal(t, commitHash2, getDSRevisionCommitHash(t, dsRevSub2))
		requireNGRKeyExists(t, ngrBucket, ngRevHash2)

		// dsRev2 should not reference ngID1.
		require.Nil(t, dsRevSub2.Get(dsRevImportedDsKey(ngID1)))
		require.Nil(t, dsRevSub2.Get(dsRevAllNGsKey(ngID1)))

		return nil
	})
	require.NoError(t, err)
}

// Test_RemoteToRemote_BBolt_ImportedNGs verifies the bbolt structure after syncing
// two NamedGraphs where one imports the other.
func Test_RemoteToRemote_BBolt_ImportedNGs(t *testing.T) {
	ngID_A := uuid.MustParse("d4e5f6a7-b8c9-0123-defa-456789012345")
	ngID_B := uuid.MustParse("e5f6a7b8-c9d0-1234-efab-567890123456")

	sourceDir := t.TempDir()
	targetDir := t.TempDir()

	sourceRepo, expectedCommitHash := createLocalRepoWithImportedNGs(t, sourceDir, "Alpha", "Beta", ngID_A, ngID_B)
	sourceRepo.Close()

	os.RemoveAll(targetDir)
	targetRepo, err := sst.CreateLocalRepository(targetDir, "target@test.com", "target", true)
	require.NoError(t, err)
	targetRepo.Close()

	sourceURL := testutil.ServerServe(t, sourceDir)

	targetServer, targetURL := startRepositoryServer(t, targetDir)

	ctx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
	sourceRemote := openRemoteRepoWithURL(t, ctx, sourceURL)
	targetRemote := openRemoteRepoWithURL(t, ctx, targetURL)

	err = targetRemote.SyncFrom(ctx, sourceRemote)
	require.NoError(t, err)

	sourceRemote.Close()
	targetRemote.Close()
	require.NoError(t, targetServer.GracefulStopAndClose())

	db := openBboltReadOnly(t, targetDir)
	defer db.Close()

	err = db.View(func(tx *bbolt.Tx) error {
		ngrBucket := tx.Bucket([]byte(bucketNamedGraphRevisions))
		require.NotNil(t, ngrBucket)
		require.Equal(t, 2, countBucketKeys(ngrBucket))

		dsrBucket := tx.Bucket([]byte(bucketDatasetRevisions))
		require.NotNil(t, dsrBucket)
		require.Equal(t, 2, countBucketKeys(dsrBucket))

		dsBucket := tx.Bucket([]byte(bucketDatasets))
		require.NotNil(t, dsBucket)
		require.Equal(t, 2, countBucketKeys(dsBucket))

		commitsBucket := tx.Bucket([]byte(bucketCommits))
		require.NotNil(t, commitsBucket)
		require.Equal(t, 1, countBucketKeys(commitsBucket), "imported datasets should share a single commit")

		// Both datasets' master branch should point to the same commit.
		requireDatasetBranchPointer(t, dsBucket, ngID_A, sst.DefaultBranch, expectedCommitHash)
		requireDatasetBranchPointer(t, dsBucket, ngID_B, sst.DefaultBranch, expectedCommitHash)

		commitSub := commitsBucket.Bucket(expectedCommitHash[:])
		require.NotNil(t, commitSub)
		require.Equal(t, "commit A with import B", string(commitSub.Get([]byte("message"))))

		dsRevHashA := getCommitDSRevision(t, commitSub, ngID_A)
		dsRevHashB := getCommitDSRevision(t, commitSub, ngID_B)

		dsRevSubA := dsrBucket.Bucket(dsRevHashA[:])
		require.NotNil(t, dsRevSubA)

		ngRevHashA := getDSRevisionNGHash(t, dsRevSubA)
		requireNGRKeyExists(t, ngrBucket, ngRevHashA)
		require.Equal(t, expectedCommitHash, getDSRevisionCommitHash(t, dsRevSubA))

		// dsRev_A must reference the imported dataset B.
		importedDsRevB := dsRevSubA.Get(dsRevImportedDsKey(ngID_B))
		require.Equal(t, dsRevHashB[:], importedDsRevB, "dsRev_A should reference dsRev_B as imported")

		// dsRev_A all-NGs list contains only imported NGs (not the default NG).
		ngRevHashB := getDSRevisionNGHash(t, dsrBucket.Bucket(dsRevHashB[:]))
		requireNGRKeyExists(t, ngrBucket, ngRevHashB)

		allNG_B := dsRevSubA.Get(dsRevAllNGsKey(ngID_B))
		require.Equal(t, ngRevHashB[:], allNG_B, "dsRev_A all-NGs list should contain imported ng_B")

		// dsRev_A's own default NG is stored under key [0x00], already verified above.
		require.Nil(t, dsRevSubA.Get(dsRevAllNGsKey(ngID_A)), "dsRev_A all-NGs list should not contain its own default ng_A")

		// dsRev_B must NOT reference ds_A (B does not import A).
		dsRevSubB := dsrBucket.Bucket(dsRevHashB[:])
		require.NotNil(t, dsRevSubB)
		require.Nil(t, dsRevSubB.Get(dsRevImportedDsKey(ngID_A)), "dsRev_B should not contain imported key for ngID_A")
		require.Nil(t, dsRevSubB.Get(dsRevAllNGsKey(ngID_A)), "dsRev_B should not contain all-NGs key for ngID_A")

		return nil
	})
	require.NoError(t, err)
}

// Test_RemoteToRemote_BBolt_SameDatasetDifferentContent verifies the bbolt structure
// after syncing a dataset that already exists in the target repository with different
// content. The current simplified merge strategy preserves the target branch pointer
// but makes the source commit history available.
func Test_RemoteToRemote_BBolt_SameDatasetDifferentContent(t *testing.T) {
	ngID := uuid.MustParse("f6a7b8c9-d0e1-2345-fabc-678901234567")

	sourceDir := t.TempDir()
	targetDir := t.TempDir()

	sourceCommitHash, sourceRepo := createLocalRepoWithOneNGAndCommit(t, sourceDir, "Alpha", ngID)
	sourceRepo.Close()

	targetCommitHash, targetRepo := createLocalRepoWithOneNGAndCommit(t, targetDir, "Beta", ngID)
	targetRepo.Close()

	sourceURL := testutil.ServerServe(t, sourceDir)

	targetServer, targetURL := startRepositoryServer(t, targetDir)

	ctx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
	sourceRemote := openRemoteRepoWithURL(t, ctx, sourceURL)
	targetRemote := openRemoteRepoWithURL(t, ctx, targetURL)

	err := targetRemote.SyncFrom(ctx, sourceRemote)
	require.NoError(t, err)

	sourceRemote.Close()
	targetRemote.Close()
	require.NoError(t, targetServer.GracefulStopAndClose())

	db := openBboltReadOnly(t, targetDir)
	defer db.Close()

	err = db.View(func(tx *bbolt.Tx) error {
		ngrBucket := tx.Bucket([]byte(bucketNamedGraphRevisions))
		require.NotNil(t, ngrBucket)
		require.Equal(t, 2, countBucketKeys(ngrBucket), "NamedGraphRevisions bucket should contain both source and target NG revisions")

		dsrBucket := tx.Bucket([]byte(bucketDatasetRevisions))
		require.NotNil(t, dsrBucket)
		require.Equal(t, 2, countBucketKeys(dsrBucket), "DatasetRevisions bucket should contain both source and target DS revisions")

		dsBucket := tx.Bucket([]byte(bucketDatasets))
		require.NotNil(t, dsBucket)
		require.Equal(t, 1, countBucketKeys(dsBucket), "Datasets bucket should contain exactly one dataset")

		commitsBucket := tx.Bucket([]byte(bucketCommits))
		require.NotNil(t, commitsBucket)
		require.Equal(t, 2, countBucketKeys(commitsBucket), "Commits bucket should contain both source and target commits")

		// The target's existing master branch pointer must be preserved.
		requireDatasetBranchPointer(t, dsBucket, ngID, sst.DefaultBranch, targetCommitHash)

		// Source commit must be present in the Commits bucket.
		require.NotNil(t, commitsBucket.Bucket(sourceCommitHash[:]), "source commit should be present in target Commits bucket")

		// Verify the source commit structure and cross-reference.
		sourceCommitSub := commitsBucket.Bucket(sourceCommitHash[:])
		require.Equal(t, "commit Alpha", string(sourceCommitSub.Get([]byte("message"))))
		sourceDSRevHash := getCommitDSRevision(t, sourceCommitSub, ngID)
		sourceDSRevSub := dsrBucket.Bucket(sourceDSRevHash[:])
		require.NotNil(t, sourceDSRevSub)
		sourceNGRevHash := getDSRevisionNGHash(t, sourceDSRevSub)
		requireNGRKeyExists(t, ngrBucket, sourceNGRevHash)
		require.Equal(t, sourceCommitHash, getDSRevisionCommitHash(t, sourceDSRevSub))

		// Verify the target commit structure and cross-reference.
		targetCommitSub := commitsBucket.Bucket(targetCommitHash[:])
		require.Equal(t, "commit Beta", string(targetCommitSub.Get([]byte("message"))))
		targetDSRevHash := getCommitDSRevision(t, targetCommitSub, ngID)
		targetDSRevSub := dsrBucket.Bucket(targetDSRevHash[:])
		require.NotNil(t, targetDSRevSub)
		targetNGRevHash := getDSRevisionNGHash(t, targetDSRevSub)
		requireNGRKeyExists(t, ngrBucket, targetNGRevHash)
		require.Equal(t, targetCommitHash, getDSRevisionCommitHash(t, targetDSRevSub))

		// The two commits must reference different dataset revisions.
		require.NotEqual(t, sourceDSRevHash, targetDSRevHash)

		return nil
	})
	require.NoError(t, err)
}

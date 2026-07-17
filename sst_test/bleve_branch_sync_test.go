// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/semanticstep/sst-core/defaultderive"
	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/sst_test/testutil"
	"github.com/semanticstep/sst-core/sstauth"
	_ "github.com/semanticstep/sst-core/vocabularies/dict"
	"github.com/semanticstep/sst-core/vocabularies/lci"
	"github.com/semanticstep/sst-core/vocabularies/rdf"
	"github.com/semanticstep/sst-core/vocabularies/rdfs"
	"github.com/semanticstep/sst-core/vocabularies/rep"
	"github.com/blevesearch/bleve/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// searchLabel helper performs a MatchQuery on the "label" field and returns the total hits.
func searchLabel(t *testing.T, index bleve.Index, ctx context.Context, term string) uint64 {
	t.Helper()
	matchQuery := bleve.NewMatchQuery(term)
	matchQuery.SetField("label")
	searchRequest := bleve.NewSearchRequest(matchQuery)
	searchRequest.Fields = []string{"label", "mainType"}

	searchResults, err := index.SearchInContext(ctx, searchRequest)
	require.NoError(t, err)
	return searchResults.Total
}

// waitForSearchLabel polls the Bleve index until a MatchQuery for term on the "label"
// field returns the expected number of hits, or the timeout is reached.
func waitForSearchLabel(t *testing.T, index bleve.Index, ctx context.Context, term string, expected uint64, timeout time.Duration, msgAndArgs ...interface{}) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if got := searchLabel(t, index, ctx, term); got == expected {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	require.Equal(t, expected, searchLabel(t, index, ctx, term), msgAndArgs...)
}

// createLocalRepoWithMasterCommit creates a new LocalFullRepository containing a single
// dataset (ngID) with one IRI node whose rdfs:label is label on the master branch.
func createLocalRepoWithMasterCommit(t *testing.T, dir, label string, ngID uuid.UUID) sst.Repository {
	t.Helper()
	repo, err := sst.CreateLocalRepository(dir, "source@test.com", "source", true)
	require.NoError(t, err)

	st := repo.OpenStage(sst.DefaultTriplexMode)
	graph := st.CreateNamedGraph(sst.IRI(ngID.URN()))
	node := graph.CreateIRINode("item", lci.Organization)
	node.AddStatement(rdfs.Label, sst.String(label))
	_, _, err = st.Commit(context.TODO(), "commit "+label, sst.DefaultBranch)
	require.NoError(t, err)
	return repo
}

// commitLabelToMaster checks out the master branch of repo (for dataset ngID), replaces the
// "item" node's triples with rdf:type lci:Organization and the given rdfs:label, and commits.
func commitLabelToMaster(t *testing.T, repo sst.Repository, label string, ngID uuid.UUID) {
	t.Helper()
	ctx := context.TODO()
	ds, err := repo.Dataset(ctx, sst.IRI(ngID.URN()))
	require.NoError(t, err)
	st, err := ds.CheckoutBranch(ctx, sst.DefaultBranch, sst.DefaultTriplexMode)
	require.NoError(t, err)
	graph := st.NamedGraph(sst.IRI(ngID.URN()))
	require.NotNil(t, graph)
	node := graph.GetIRINodeByFragment("item")
	require.NotNil(t, node)
	node.DeleteTriples()
	node.AddStatement(rdf.Type, lci.Organization)
	node.AddStatement(rdfs.Label, sst.String(label))
	_, _, err = st.Commit(ctx, "commit "+label, sst.DefaultBranch)
	require.NoError(t, err)
}

// createFeatureBranch checks out master for dataset ngID, creates a new "feature-item" node
// with the given label, commits to branchName, and returns the commit hash.
func createFeatureBranch(t *testing.T, repo sst.Repository, label string, branchName string, ngID uuid.UUID) sst.Hash {
	t.Helper()
	ctx := context.TODO()
	ds, err := repo.Dataset(ctx, sst.IRI(ngID.URN()))
	require.NoError(t, err)
	st, err := ds.CheckoutBranch(ctx, sst.DefaultBranch, sst.DefaultTriplexMode)
	require.NoError(t, err)
	graph := st.NamedGraph(sst.IRI(ngID.URN()))
	require.NotNil(t, graph)
	node := graph.CreateIRINode("feature-item", lci.Organization)
	node.AddStatement(rdfs.Label, sst.String(label))
	commitHash, _, err := st.Commit(ctx, "commit "+label, branchName)
	require.NoError(t, err)
	return commitHash
}

// Test_LocalFullRepository_BleveIndex_BranchSync verifies that the Bleve index stays in sync
// with the master branch across Commit, SetBranch and RemoveBranch operations.
func Test_LocalFullRepository_BleveIndex_BranchSync(t *testing.T) {
	testName := t.Name() + "Repo"
	dir := filepath.Join("./testdata/", testName)
	ngID := uuid.MustParse("a1b2c3d4-e5f6-7890-abcd-ef1234567890")

	defer os.RemoveAll(dir)

	t.Run("commit_sync", func(t *testing.T) {
		removeFolder(dir)

		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
		require.NoError(t, err)
		defer repo.Close()

		err = repo.RegisterIndexHandler(defaultderive.DeriveInfo())
		require.NoError(t, err)

		ctx := context.TODO()
		index := repo.Bleve()
		require.NotNil(t, index)

		// 1. Before any commit → index is empty
		waitForSearchLabel(t, index, ctx, "Alpha", uint64(0), 5*time.Second)

		// 2. Commit to master → index updated
		st := repo.OpenStage(sst.DefaultTriplexMode)
		graph := st.CreateNamedGraph(sst.IRI(ngID.URN()))
		node := graph.CreateIRINode("item", lci.Organization)
		node.AddStatement(rdfs.Label, sst.String("Alpha"))
		node.AddStatement(rdf.Type, rep.SchematicPort)
		_, _, err = st.Commit(ctx, "first commit", sst.DefaultBranch)
		require.NoError(t, err)
		sst.FlushBleveIndex(repo)

		waitForSearchLabel(t, index, ctx, "Alpha", uint64(1), 5*time.Second)

		// 3. Commit to a non-master branch → master index must NOT change
		ds, err := repo.Dataset(ctx, sst.IRI(ngID.URN()))
		require.NoError(t, err)
		st2, err := ds.CheckoutBranch(ctx, sst.DefaultBranch, sst.DefaultTriplexMode)
		require.NoError(t, err)
		graph2 := st2.NamedGraph(sst.IRI(ngID.URN()))
		require.NotNil(t, graph2)
		node2 := graph2.GetIRINodeByFragment("item")
		require.NotNil(t, node2)
		node2.DeleteTriples()
		node2.AddStatement(rdf.Type, lci.Organization)
		node2.AddStatement(rdfs.Label, sst.String("Beta"))
		_, _, err = st2.Commit(ctx, "feature commit", "feature")
		require.NoError(t, err)
		sst.FlushBleveIndex(repo)

		waitForSearchLabel(t, index, ctx, "Alpha", uint64(1), 5*time.Second, "master index should still contain Alpha")
		waitForSearchLabel(t, index, ctx, "Beta", uint64(0), 5*time.Second, "master index should not contain Beta")

		// 4. Another commit to master → index updated to new master data
		st3, err := ds.CheckoutBranch(ctx, sst.DefaultBranch, sst.DefaultTriplexMode)
		require.NoError(t, err)
		graph3 := st3.NamedGraph(sst.IRI(ngID.URN()))
		require.NotNil(t, graph3)
		node3 := graph3.GetIRINodeByFragment("item")
		require.NotNil(t, node3)
		node3.DeleteTriples()
		node3.AddStatement(rdf.Type, lci.Organization)
		node3.AddStatement(rdfs.Label, sst.String("Gamma"))
		_, _, err = st3.Commit(ctx, "second master commit", sst.DefaultBranch)
		require.NoError(t, err)
		sst.FlushBleveIndex(repo)

		waitForSearchLabel(t, index, ctx, "Alpha", uint64(0), 5*time.Second, "master index should no longer contain Alpha")
		waitForSearchLabel(t, index, ctx, "Gamma", uint64(1), 5*time.Second, "master index should contain Gamma")
	})

	t.Run("setBranch_sync", func(t *testing.T) {
		removeFolder(dir)

		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
		require.NoError(t, err)
		defer repo.Close()

		err = repo.RegisterIndexHandler(defaultderive.DeriveInfo())
		require.NoError(t, err)

		ctx := context.TODO()
		index := repo.Bleve()
		require.NotNil(t, index)

		// commit1(master): label = Alpha
		st := repo.OpenStage(sst.DefaultTriplexMode)
		graph := st.CreateNamedGraph(sst.IRI(ngID.URN()))
		node := graph.CreateIRINode("item", lci.Organization)
		node.AddStatement(rdfs.Label, sst.String("Alpha"))
		node.AddStatement(rdf.Type, rep.SchematicPort)
		commit1, _, err := st.Commit(ctx, "commit Alpha", sst.DefaultBranch)
		require.NoError(t, err)
		sst.FlushBleveIndex(repo)

		// Create an extra branch pointing to commit1 so that commit1 remains in the dataset bucket.
		// Otherwise commit2 would overwrite master and commit1 would not be found by SetBranch.
		ds, err := repo.Dataset(ctx, sst.IRI(ngID.URN()))
		require.NoError(t, err)
		err = ds.SetBranchCommit(ctx, commit1, "preserve")
		require.NoError(t, err)
		sst.FlushBleveIndex(repo)

		// commit2(master): label = Beta
		st2, err := ds.CheckoutBranch(ctx, sst.DefaultBranch, sst.DefaultTriplexMode)
		require.NoError(t, err)
		graph2 := st2.NamedGraph(sst.IRI(ngID.URN()))
		require.NotNil(t, graph2)
		node2 := graph2.GetIRINodeByFragment("item")
		require.NotNil(t, node2)
		node2.DeleteTriples()
		node2.AddStatement(rdf.Type, lci.Organization)
		node2.AddStatement(rdfs.Label, sst.String("Beta"))
		_, _, err = st2.Commit(ctx, "commit Beta", sst.DefaultBranch)
		require.NoError(t, err)
		sst.FlushBleveIndex(repo)

		waitForSearchLabel(t, index, ctx, "Alpha", uint64(0), 5*time.Second)
		waitForSearchLabel(t, index, ctx, "Beta", uint64(1), 5*time.Second)

		// Set master branch back to commit1 → index should reflect Alpha again
		err = ds.SetBranchCommit(ctx, commit1, sst.DefaultBranch)
		require.NoError(t, err)
		sst.FlushBleveIndex(repo)

		waitForSearchLabel(t, index, ctx, "Alpha", uint64(1), 5*time.Second, "after SetBranch to commit1, Alpha should be searchable")
		waitForSearchLabel(t, index, ctx, "Beta", uint64(0), 5*time.Second, "after SetBranch to commit1, Beta should not be searchable")
	})

	t.Run("removeBranch_sync", func(t *testing.T) {
		removeFolder(dir)

		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
		require.NoError(t, err)
		defer repo.Close()

		err = repo.RegisterIndexHandler(defaultderive.DeriveInfo())
		require.NoError(t, err)

		ctx := context.TODO()
		index := repo.Bleve()
		require.NotNil(t, index)

		st := repo.OpenStage(sst.DefaultTriplexMode)
		graph := st.CreateNamedGraph(sst.IRI(ngID.URN()))
		node := graph.CreateIRINode("item", lci.Organization)
		node.AddStatement(rdfs.Label, sst.String("Alpha"))
		node.AddStatement(rdf.Type, rep.SchematicPort)
		_, _, err = st.Commit(ctx, "commit Alpha", sst.DefaultBranch)
		require.NoError(t, err)
		sst.FlushBleveIndex(repo)

		waitForSearchLabel(t, index, ctx, "Alpha", uint64(1), 5*time.Second)

		ds, err := repo.Dataset(ctx, sst.IRI(ngID.URN()))
		require.NoError(t, err)
		err = ds.RemoveBranch(ctx, sst.DefaultBranch)
		require.NoError(t, err)
		sst.FlushBleveIndex(repo)

		waitForSearchLabel(t, index, ctx, "Alpha", uint64(0), 5*time.Second, "after RemoveBranch(master), index should be empty")
	})
}

// Test_LocalFullRepository_BleveIndex_SyncFrom verifies that SyncFrom keeps the Bleve index
// in sync with the master branch for both new and existing target datasets.
func Test_LocalFullRepository_BleveIndex_SyncFrom(t *testing.T) {
	ngID := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a400")

	t.Run("new_dataset", func(t *testing.T) {
		sourceDir := filepath.Join(t.TempDir(), "repo")
		sourceRepo := createLocalRepoWithMasterCommit(t, sourceDir, "Alpha", ngID)
		defer sourceRepo.Close()

		targetDir := filepath.Join(t.TempDir(), "repo")
		targetRepo, err := sst.CreateLocalRepository(targetDir, "target@test.com", "target", true)
		require.NoError(t, err)
		defer targetRepo.Close()
		err = targetRepo.RegisterIndexHandler(defaultderive.DeriveInfo())
		require.NoError(t, err)

		ctx := context.TODO()
		err = targetRepo.SyncFrom(ctx, sourceRepo)
		require.NoError(t, err)
		sst.FlushBleveIndex(targetRepo)

		index := targetRepo.Bleve()
		require.NotNil(t, index)
		waitForSearchLabel(t, index, ctx, "Alpha", uint64(1), 5*time.Second, "index should contain Alpha after initial sync")
	})

	t.Run("existing_dataset_master_updated", func(t *testing.T) {
		sourceDir := filepath.Join(t.TempDir(), "repo")
		sourceRepo := createLocalRepoWithMasterCommit(t, sourceDir, "Alpha", ngID)
		defer sourceRepo.Close()

		targetDir := filepath.Join(t.TempDir(), "repo")
		targetRepo, err := sst.CreateLocalRepository(targetDir, "target@test.com", "target", true)
		require.NoError(t, err)
		defer targetRepo.Close()
		err = targetRepo.RegisterIndexHandler(defaultderive.DeriveInfo())
		require.NoError(t, err)

		ctx := context.TODO()
		err = targetRepo.SyncFrom(ctx, sourceRepo)
		require.NoError(t, err)
		sst.FlushBleveIndex(targetRepo)

		index := targetRepo.Bleve()
		require.NotNil(t, index)
		waitForSearchLabel(t, index, ctx, "Alpha", uint64(1), 5*time.Second, "index should contain Alpha after initial sync")

		// Update master branch on source and sync only the master branch again.
		commitLabelToMaster(t, sourceRepo, "Beta", ngID)
		err = targetRepo.SyncFrom(ctx, sourceRepo, sst.WithBranch(sst.DefaultBranch))
		require.NoError(t, err)
		sst.FlushBleveIndex(targetRepo)

		waitForSearchLabel(t, index, ctx, "Alpha", uint64(0), 5*time.Second, "index should no longer contain Alpha")
		waitForSearchLabel(t, index, ctx, "Beta", uint64(1), 5*time.Second, "index should contain Beta after re-sync")
	})

	t.Run("non_master_branch_does_not_update_master_index", func(t *testing.T) {
		sourceDir := filepath.Join(t.TempDir(), "repo")
		sourceRepo := createLocalRepoWithMasterCommit(t, sourceDir, "Alpha", ngID)
		defer sourceRepo.Close()

		// Create a feature branch with a different label.
		createFeatureBranch(t, sourceRepo, "Beta", "feature", ngID)

		targetDir := filepath.Join(t.TempDir(), "repo")
		targetRepo, err := sst.CreateLocalRepository(targetDir, "target@test.com", "target", true)
		require.NoError(t, err)
		defer targetRepo.Close()
		err = targetRepo.RegisterIndexHandler(defaultderive.DeriveInfo())
		require.NoError(t, err)

		ctx := context.TODO()
		// Sync only the feature branch.
		err = targetRepo.SyncFrom(ctx, sourceRepo, sst.WithBranch("feature"))
		require.NoError(t, err)
		sst.FlushBleveIndex(targetRepo)

		index := targetRepo.Bleve()
		require.NotNil(t, index)
		waitForSearchLabel(t, index, ctx, "Alpha", uint64(0), 5*time.Second, "master index should not contain Alpha after feature-only sync")
		waitForSearchLabel(t, index, ctx, "Beta", uint64(0), 5*time.Second, "master index should not contain Beta after feature-only sync")
	})

	t.Run("target_already_has_content", func(t *testing.T) {
		existingNgID := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a410")

		// Target already contains a dataset with label "Existing".
		targetDir := filepath.Join(t.TempDir(), "repo")
		targetRepo := createLocalRepoWithMasterCommit(t, targetDir, "Existing", existingNgID)
		err := targetRepo.RegisterIndexHandler(defaultderive.DeriveInfo())
		require.NoError(t, err)
		sst.FlushBleveIndex(targetRepo)
		defer targetRepo.Close()

		// Source contains a different dataset with label "Alpha".
		sourceDir := filepath.Join(t.TempDir(), "repo")
		sourceRepo := createLocalRepoWithMasterCommit(t, sourceDir, "Alpha", ngID)
		defer sourceRepo.Close()

		ctx := context.TODO()
		err = targetRepo.SyncFrom(ctx, sourceRepo)
		require.NoError(t, err)
		sst.FlushBleveIndex(targetRepo)

		index := targetRepo.Bleve()
		require.NotNil(t, index)
		waitForSearchLabel(t, index, ctx, "Existing", uint64(1), 5*time.Second, "target index should still contain the pre-existing dataset label")
		waitForSearchLabel(t, index, ctx, "Alpha", uint64(1), 5*time.Second, "target index should contain the newly synced dataset label")
	})
}

// Test_RemoteRepository_BleveIndex_BranchSync verifies the same sync behavior via RemoteRepository (gRPC).
func Test_RemoteRepository_BleveIndex_BranchSync(t *testing.T) {
	testName := t.Name() + "Repo"
	dir := filepath.Join("./testdata/", testName)
	ngID := uuid.MustParse("b2c3d4e5-f6a7-8901-bcde-f23456789012")

	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)

	defer os.RemoveAll(dir)

	t.Run("commit_sync", func(t *testing.T) {
		removeFolder(dir)
		subDir := filepath.Join(dir, "commit")
		url := testutil.ServerServe(t, subDir)
		ctx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

		repo, err := sst.OpenRemoteRepository(ctx, url, transportCreds)
		require.NoError(t, err)
		defer repo.Close()

		index := repo.Bleve()
		require.NotNil(t, index)

		// 1. Before any commit → index is empty
		waitForSearchLabel(t, index, ctx, "Alpha", uint64(0), 5*time.Second)

		// 2. Commit to master → index updated
		st := repo.OpenStage(sst.DefaultTriplexMode)
		graph := st.CreateNamedGraph(sst.IRI(ngID.URN()))
		node := graph.CreateIRINode("item", lci.Organization)
		node.AddStatement(rdfs.Label, sst.String("Alpha"))
		node.AddStatement(rdf.Type, rep.SchematicPort)
		_, _, err = st.Commit(ctx, "first commit", sst.DefaultBranch)
		require.NoError(t, err)

		waitForSearchLabel(t, index, ctx, "Alpha", uint64(1), 5*time.Second)

		// 3. Commit to a non-master branch → master index must NOT change
		ds, err := repo.Dataset(ctx, sst.IRI(ngID.URN()))
		require.NoError(t, err)
		st2, err := ds.CheckoutBranch(ctx, sst.DefaultBranch, sst.DefaultTriplexMode)
		require.NoError(t, err)
		graph2 := st2.NamedGraph(sst.IRI(ngID.URN()))
		require.NotNil(t, graph2)
		node2 := graph2.GetIRINodeByFragment("item")
		require.NotNil(t, node2)
		node2.DeleteTriples()
		node2.AddStatement(rdf.Type, lci.Organization)
		node2.AddStatement(rdfs.Label, sst.String("Beta"))
		_, _, err = st2.Commit(ctx, "feature commit", "feature")
		require.NoError(t, err)

		waitForSearchLabel(t, index, ctx, "Alpha", uint64(1), 5*time.Second, "master index should still contain Alpha")
		waitForSearchLabel(t, index, ctx, "Beta", uint64(0), 5*time.Second, "master index should not contain Beta")

		// 4. Another commit to master → index updated to new master data
		st3, err := ds.CheckoutBranch(ctx, sst.DefaultBranch, sst.DefaultTriplexMode)
		require.NoError(t, err)
		graph3 := st3.NamedGraph(sst.IRI(ngID.URN()))
		require.NotNil(t, graph3)
		node3 := graph3.GetIRINodeByFragment("item")
		require.NotNil(t, node3)
		node3.DeleteTriples()
		node3.AddStatement(rdf.Type, lci.Organization)
		node3.AddStatement(rdfs.Label, sst.String("Gamma"))
		_, _, err = st3.Commit(ctx, "second master commit", sst.DefaultBranch)
		require.NoError(t, err)

		waitForSearchLabel(t, index, ctx, "Alpha", uint64(0), 5*time.Second, "master index should no longer contain Alpha")
		waitForSearchLabel(t, index, ctx, "Gamma", uint64(1), 5*time.Second, "master index should contain Gamma")
	})

	t.Run("setBranch_sync", func(t *testing.T) {
		removeFolder(dir)
		subDir := filepath.Join(dir, "setbranch")
		url := testutil.ServerServe(t, subDir)
		ctx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

		repo, err := sst.OpenRemoteRepository(ctx, url, transportCreds)
		require.NoError(t, err)
		defer repo.Close()

		index := repo.Bleve()
		require.NotNil(t, index)

		// commit1(master): label = Alpha
		st := repo.OpenStage(sst.DefaultTriplexMode)
		graph := st.CreateNamedGraph(sst.IRI(ngID.URN()))
		node := graph.CreateIRINode("item", lci.Organization)
		node.AddStatement(rdfs.Label, sst.String("Alpha"))
		node.AddStatement(rdf.Type, rep.SchematicPort)
		commit1, _, err := st.Commit(ctx, "commit Alpha", sst.DefaultBranch)
		require.NoError(t, err)

		// Create an extra branch pointing to commit1 so that commit1 remains in the dataset bucket.
		ds, err := repo.Dataset(ctx, sst.IRI(ngID.URN()))
		require.NoError(t, err)
		err = ds.SetBranchCommit(ctx, commit1, "preserve")
		require.NoError(t, err)

		// commit2(master): label = Beta
		st2, err := ds.CheckoutBranch(ctx, sst.DefaultBranch, sst.DefaultTriplexMode)
		require.NoError(t, err)
		graph2 := st2.NamedGraph(sst.IRI(ngID.URN()))
		require.NotNil(t, graph2)
		node2 := graph2.GetIRINodeByFragment("item")
		require.NotNil(t, node2)
		node2.DeleteTriples()
		node2.AddStatement(rdf.Type, lci.Organization)
		node2.AddStatement(rdfs.Label, sst.String("Beta"))
		_, _, err = st2.Commit(ctx, "commit Beta", sst.DefaultBranch)
		require.NoError(t, err)

		waitForSearchLabel(t, index, ctx, "Alpha", uint64(0), 5*time.Second)
		waitForSearchLabel(t, index, ctx, "Beta", uint64(1), 5*time.Second)

		// Set master branch back to commit1 → index should reflect Alpha again
		err = ds.SetBranchCommit(ctx, commit1, sst.DefaultBranch)
		require.NoError(t, err)

		waitForSearchLabel(t, index, ctx, "Alpha", uint64(1), 5*time.Second, "after SetBranch to commit1, Alpha should be searchable")
		waitForSearchLabel(t, index, ctx, "Beta", uint64(0), 5*time.Second, "after SetBranch to commit1, Beta should not be searchable")
	})

	t.Run("removeBranch_sync", func(t *testing.T) {
		removeFolder(dir)
		subDir := filepath.Join(dir, "removebranch")
		url := testutil.ServerServe(t, subDir)
		ctx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

		repo, err := sst.OpenRemoteRepository(ctx, url, transportCreds)
		require.NoError(t, err)
		defer repo.Close()

		index := repo.Bleve()
		require.NotNil(t, index)

		st := repo.OpenStage(sst.DefaultTriplexMode)
		graph := st.CreateNamedGraph(sst.IRI(ngID.URN()))
		node := graph.CreateIRINode("item", lci.Organization)
		node.AddStatement(rdfs.Label, sst.String("Alpha"))
		node.AddStatement(rdf.Type, rep.SchematicPort)
		_, _, err = st.Commit(ctx, "commit Alpha", sst.DefaultBranch)
		require.NoError(t, err)

		waitForSearchLabel(t, index, ctx, "Alpha", uint64(1), 5*time.Second)

		ds, err := repo.Dataset(ctx, sst.IRI(ngID.URN()))
		require.NoError(t, err)
		err = ds.RemoveBranch(ctx, sst.DefaultBranch)
		require.NoError(t, err)

		waitForSearchLabel(t, index, ctx, "Alpha", uint64(0), 5*time.Second, "after RemoveBranch(master), index should be empty")
	})
}

// Test_RemoteRepository_BleveIndex_SyncFrom verifies that SyncFrom into a RemoteRepository
// (local → remote via gRPC) updates the server-side Bleve index for the master branch.
func Test_RemoteRepository_BleveIndex_SyncFrom(t *testing.T) {
	ngID := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a401")

	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)

	t.Run("local_to_remote_new_dataset", func(t *testing.T) {
		sourceDir := filepath.Join(t.TempDir(), "repo")
		sourceRepo := createLocalRepoWithMasterCommit(t, sourceDir, "Gamma", ngID)
		defer sourceRepo.Close()

		targetDir := filepath.Join(t.TempDir(), "repo")
		url := testutil.ServerServe(t, targetDir)
		ctx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

		remoteRepo, err := sst.OpenRemoteRepository(ctx, url, transportCreds)
		require.NoError(t, err)
		defer remoteRepo.Close()

		err = remoteRepo.SyncFrom(ctx, sourceRepo)
		require.NoError(t, err)

		index := remoteRepo.Bleve()
		require.NotNil(t, index)
		waitForSearchLabel(t, index, ctx, "Gamma", uint64(1), 5*time.Second, "remote index should contain Gamma after sync")
	})

	t.Run("remote_to_remote_new_dataset", func(t *testing.T) {
		sourceDir := filepath.Join(t.TempDir(), "repo")
		sourceLocalRepo := createLocalRepoWithMasterCommit(t, sourceDir, "Gamma", ngID)
		sourceLocalRepo.Close() // Server will open the source repository itself.

		sourceURL := testutil.ServerServe(t, sourceDir)
		targetDir := filepath.Join(t.TempDir(), "repo")
		targetURL := testutil.ServerServe(t, targetDir)

		ctx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

		sourceRemoteRepo, err := sst.OpenRemoteRepository(ctx, sourceURL, transportCreds)
		require.NoError(t, err)
		defer sourceRemoteRepo.Close()

		targetRemoteRepo, err := sst.OpenRemoteRepository(ctx, targetURL, transportCreds)
		require.NoError(t, err)
		defer targetRemoteRepo.Close()

		err = targetRemoteRepo.SyncFrom(ctx, sourceRemoteRepo)
		require.NoError(t, err)

		index := targetRemoteRepo.Bleve()
		require.NotNil(t, index)
		waitForSearchLabel(t, index, ctx, "Gamma", uint64(1), 5*time.Second, "target remote index should contain Gamma after remote-to-remote sync")
	})

	t.Run("remote_to_remote_target_has_content", func(t *testing.T) {
		existingNgID := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a411")

		// Target server already contains a dataset with label "Existing".
		targetDir := filepath.Join(t.TempDir(), "repo")
		targetLocalRepo := createLocalRepoWithMasterCommit(t, targetDir, "Existing", existingNgID)
		targetLocalRepo.Close() // Server will open the target repository itself.

		targetURL := testutil.ServerServe(t, targetDir)

		// Source server contains a different dataset with label "Gamma".
		sourceDir := filepath.Join(t.TempDir(), "repo")
		sourceLocalRepo := createLocalRepoWithMasterCommit(t, sourceDir, "Gamma", ngID)
		sourceLocalRepo.Close() // Server will open the source repository itself.

		sourceURL := testutil.ServerServe(t, sourceDir)

		ctx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

		sourceRemoteRepo, err := sst.OpenRemoteRepository(ctx, sourceURL, transportCreds)
		require.NoError(t, err)
		defer sourceRemoteRepo.Close()

		targetRemoteRepo, err := sst.OpenRemoteRepository(ctx, targetURL, transportCreds)
		require.NoError(t, err)
		defer targetRemoteRepo.Close()

		err = targetRemoteRepo.SyncFrom(ctx, sourceRemoteRepo)
		require.NoError(t, err)

		index := targetRemoteRepo.Bleve()
		require.NotNil(t, index)
		waitForSearchLabel(t, index, ctx, "Existing", uint64(1), 5*time.Second, "target remote index should still contain the pre-existing dataset label")
		waitForSearchLabel(t, index, ctx, "Gamma", uint64(1), 5*time.Second, "target remote index should contain the newly synced dataset label")
	})

	// Note: remote-to-remote sync currently uses a simplified merge that keeps
	// existing branch values when they differ, so updating an existing master
	// branch is not covered here. That scenario is tested for local-to-local sync.
}

// Test_RemoteToLocal_BleveIndex_SyncFrom verifies that SyncFrom a RemoteRepository into a
// LocalFullRepository updates the local Bleve index for the master branch.
func Test_RemoteToLocal_BleveIndex_SyncFrom(t *testing.T) {
	ngID := uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a402")

	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)

	t.Run("remote_to_local_new_dataset", func(t *testing.T) {
		remoteDir := filepath.Join(t.TempDir(), "repo")
		remoteRepo := createLocalRepoWithMasterCommit(t, remoteDir, "Epsilon", ngID)
		remoteRepo.Close() // Server will open the repository itself.

		url := testutil.ServerServe(t, remoteDir)
		ctx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

		remoteClient, err := sst.OpenRemoteRepository(ctx, url, transportCreds)
		require.NoError(t, err)
		defer remoteClient.Close()

		targetDir := filepath.Join(t.TempDir(), "repo")
		targetRepo, err := sst.CreateLocalRepository(targetDir, "target@test.com", "target", true)
		require.NoError(t, err)
		defer targetRepo.Close()
		err = targetRepo.RegisterIndexHandler(defaultderive.DeriveInfo())
		require.NoError(t, err)

		err = targetRepo.SyncFrom(ctx, remoteClient)
		require.NoError(t, err)
		sst.FlushBleveIndex(targetRepo)

		index := targetRepo.Bleve()
		require.NotNil(t, index)
		waitForSearchLabel(t, index, ctx, "Epsilon", uint64(1), 5*time.Second, "local index should contain Epsilon after sync from remote")
	})

	// Note: remote-to-local sync currently uses a simplified merge that keeps
	// existing branch values when they differ, so updating an existing master
	// branch is not covered here. That scenario is tested for local-to-local sync.
}

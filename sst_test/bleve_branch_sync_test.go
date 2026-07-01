// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/blevesearch/bleve/v2"
	"github.com/google/uuid"
	"github.com/semanticstep/sst-core/defaultderive"
	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/sst_test/testutil"
	"github.com/semanticstep/sst-core/sstauth"
	_ "github.com/semanticstep/sst-core/vocabularies/dict"
	"github.com/semanticstep/sst-core/vocabularies/lci"
	"github.com/semanticstep/sst-core/vocabularies/rdf"
	"github.com/semanticstep/sst-core/vocabularies/rdfs"
	"github.com/semanticstep/sst-core/vocabularies/rep"
	"github.com/stretchr/testify/assert"
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
		assert.Equal(t, uint64(0), searchLabel(t, index, ctx, "Alpha"))

		// 2. Commit to master → index updated
		st := repo.OpenStage(sst.DefaultTriplexMode)
		graph := st.CreateNamedGraph(sst.IRI(ngID.URN()))
		node := graph.CreateIRINode("item", lci.Organization)
		node.AddStatement(rdfs.Label, sst.String("Alpha"))
		node.AddStatement(rdf.Type, rep.SchematicPort)
		_, _, err = st.Commit(ctx, "first commit", sst.DefaultBranch)
		require.NoError(t, err)
		sst.FlushBleveIndex(repo)

		assert.Equal(t, uint64(1), searchLabel(t, index, ctx, "Alpha"))

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

		assert.Equal(t, uint64(1), searchLabel(t, index, ctx, "Alpha"), "master index should still contain Alpha")
		assert.Equal(t, uint64(0), searchLabel(t, index, ctx, "Beta"), "master index should not contain Beta")

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

		assert.Equal(t, uint64(0), searchLabel(t, index, ctx, "Alpha"), "master index should no longer contain Alpha")
		assert.Equal(t, uint64(1), searchLabel(t, index, ctx, "Gamma"), "master index should contain Gamma")
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

		assert.Equal(t, uint64(0), searchLabel(t, index, ctx, "Alpha"))
		assert.Equal(t, uint64(1), searchLabel(t, index, ctx, "Beta"))

		// Set master branch back to commit1 → index should reflect Alpha again
		err = ds.SetBranchCommit(ctx, commit1, sst.DefaultBranch)
		require.NoError(t, err)
		sst.FlushBleveIndex(repo)

		assert.Equal(t, uint64(1), searchLabel(t, index, ctx, "Alpha"), "after SetBranch to commit1, Alpha should be searchable")
		assert.Equal(t, uint64(0), searchLabel(t, index, ctx, "Beta"), "after SetBranch to commit1, Beta should not be searchable")
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

		assert.Equal(t, uint64(1), searchLabel(t, index, ctx, "Alpha"))

		ds, err := repo.Dataset(ctx, sst.IRI(ngID.URN()))
		require.NoError(t, err)
		err = ds.RemoveBranch(ctx, sst.DefaultBranch)
		require.NoError(t, err)
		sst.FlushBleveIndex(repo)

		assert.Equal(t, uint64(0), searchLabel(t, index, ctx, "Alpha"), "after RemoveBranch(master), index should be empty")
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
		assert.Equal(t, uint64(0), searchLabel(t, index, ctx, "Alpha"))

		// 2. Commit to master → index updated
		st := repo.OpenStage(sst.DefaultTriplexMode)
		graph := st.CreateNamedGraph(sst.IRI(ngID.URN()))
		node := graph.CreateIRINode("item", lci.Organization)
		node.AddStatement(rdfs.Label, sst.String("Alpha"))
		node.AddStatement(rdf.Type, rep.SchematicPort)
		_, _, err = st.Commit(ctx, "first commit", sst.DefaultBranch)
		require.NoError(t, err)

		assert.Equal(t, uint64(1), searchLabel(t, index, ctx, "Alpha"))

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

		assert.Equal(t, uint64(1), searchLabel(t, index, ctx, "Alpha"), "master index should still contain Alpha")
		assert.Equal(t, uint64(0), searchLabel(t, index, ctx, "Beta"), "master index should not contain Beta")

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

		assert.Equal(t, uint64(0), searchLabel(t, index, ctx, "Alpha"), "master index should no longer contain Alpha")
		assert.Equal(t, uint64(1), searchLabel(t, index, ctx, "Gamma"), "master index should contain Gamma")
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

		assert.Equal(t, uint64(0), searchLabel(t, index, ctx, "Alpha"))
		assert.Equal(t, uint64(1), searchLabel(t, index, ctx, "Beta"))

		// Set master branch back to commit1 → index should reflect Alpha again
		err = ds.SetBranchCommit(ctx, commit1, sst.DefaultBranch)
		require.NoError(t, err)

		assert.Equal(t, uint64(1), searchLabel(t, index, ctx, "Alpha"), "after SetBranch to commit1, Alpha should be searchable")
		assert.Equal(t, uint64(0), searchLabel(t, index, ctx, "Beta"), "after SetBranch to commit1, Beta should not be searchable")
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

		assert.Equal(t, uint64(1), searchLabel(t, index, ctx, "Alpha"))

		ds, err := repo.Dataset(ctx, sst.IRI(ngID.URN()))
		require.NoError(t, err)
		err = ds.RemoveBranch(ctx, sst.DefaultBranch)
		require.NoError(t, err)

		assert.Equal(t, uint64(0), searchLabel(t, index, ctx, "Alpha"), "after RemoveBranch(master), index should be empty")
	})
}

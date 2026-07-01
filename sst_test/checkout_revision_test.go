// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/sst_test/testutil"
	"github.com/semanticstep/sst-core/sstauth"
	"github.com/semanticstep/sst-core/vocabularies/rdf"
	"github.com/semanticstep/sst-core/vocabularies/rep"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_LocalFullRepository_CheckoutRevision_Basic tests basic CheckoutRevision functionality
func Test_LocalFullRepository_CheckoutRevision_Basic(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a400").URN())

	var dsRevision1 sst.Hash

	t.Run("create_and_commit", func(t *testing.T) {

		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
		assert.NoError(t, err)
		defer repo.Close()

		st := repo.OpenStage(sst.DefaultTriplexMode)
		ng := st.CreateNamedGraph(ngIRI)
		mainNode := ng.CreateIRINode("mainNode")
		mainNode.AddStatement(rdf.Type, rep.SchematicPort)

		// First commit to default branch
		_, _, err = st.Commit(context.TODO(), "First commit", sst.DefaultBranch)
		assert.NoError(t, err)

		// Get the dataset revision from the stage
		info := ng.Info()
		dsRevision1 = info.DatasetRevision
		assert.False(t, dsRevision1.IsNil(), "DatasetRevision should not be nil after commit")
	})

	t.Run("checkout_revision", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		assert.NoError(t, err)
		defer repo.Close()

		ds, err := repo.Dataset(context.TODO(), ngIRI)
		assert.NoError(t, err)

		// Checkout by dataset revision (not by branch or commit)
		st, err := ds.CheckoutRevision(context.TODO(), dsRevision1, sst.DefaultTriplexMode)
		assert.NoError(t, err)
		assert.NotNil(t, st)

		// Verify the named graph is loaded correctly
		ng := st.NamedGraph(ngIRI)
		assert.NotNil(t, ng)

		// Verify the data is present
		mainNode := ng.GetIRINodeByFragment("mainNode")
		assert.NotNil(t, mainNode)

		// Verify the checkout info
		info := ng.Info()
		assert.Equal(t, dsRevision1, info.DatasetRevision, "DatasetRevision should match")
		// For CheckoutRevision, commits should be populated from the dataset revision
		assert.NotEmpty(t, info.CheckedOutCommits, "Commits should be populated for revision checkout")
	})

	t.Run("checkout_revision_and_commit_to_new_branch", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		assert.NoError(t, err)
		defer repo.Close()

		ds, err := repo.Dataset(context.TODO(), ngIRI)
		assert.NoError(t, err)

		// Checkout by dataset revision
		st, err := ds.CheckoutRevision(context.TODO(), dsRevision1, sst.DefaultTriplexMode)
		assert.NoError(t, err)

		ng := st.NamedGraph(ngIRI)
		mainNode := ng.GetIRINodeByFragment("mainNode")
		mainNode.AddStatement(rdf.Bag, rep.Angle)

		// Commit to a new branch
		commitHash2, _, err := st.Commit(context.TODO(), "Second commit from revision checkout", "revisionBranch")
		assert.NoError(t, err)
		assert.False(t, commitHash2.IsNil(), "New commit should be created")

		// Verify the new branch exists
		branches, err := ds.Branches(context.TODO())
		assert.NoError(t, err)
		assert.Equal(t, commitHash2, branches["revisionBranch"], "New branch should point to new commit")
	})
}

// Test_LocalFullRepository_CheckoutRevision_WithImports tests CheckoutRevision with imported datasets
func Test_LocalFullRepository_CheckoutRevision_WithImports(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngBaseIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a401").URN())
	ngImportIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a402").URN())

	var baseDSRevision sst.Hash

	t.Run("setup_imports_and_commit", func(t *testing.T) {

		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
		assert.NoError(t, err)
		defer repo.Close()

		// Create the imported dataset first
		st := repo.OpenStage(sst.DefaultTriplexMode)
		ngImport := st.CreateNamedGraph(ngImportIRI)
		importNode := ngImport.CreateIRINode("importNode")
		importNode.AddStatement(rdf.Type, rep.SchematicPort)

		_, _, err = st.Commit(context.TODO(), "Import dataset commit", sst.DefaultBranch)
		assert.NoError(t, err)

		// Now create the base dataset that imports the first one
		// Use the same stage to avoid "stages are not the same" error
		ngBase := st.CreateNamedGraph(ngBaseIRI)
		baseNode := ngBase.CreateIRINode("baseNode")
		baseNode.AddStatement(rdf.Type, rep.SchematicPort)

		// Add import
		err = ngBase.AddImport(ngImport)
		assert.NoError(t, err)

		_, _, err = st.Commit(context.TODO(), "Base dataset with import", sst.DefaultBranch)
		assert.NoError(t, err)

		// Get the dataset revision
		info := ngBase.Info()
		baseDSRevision = info.DatasetRevision
	})

	t.Run("checkout_revision_with_imports", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		assert.NoError(t, err)
		defer repo.Close()

		ds, err := repo.Dataset(context.TODO(), ngBaseIRI)
		assert.NoError(t, err)

		// Checkout by dataset revision
		st, err := ds.CheckoutRevision(context.TODO(), baseDSRevision, sst.DefaultTriplexMode)
		assert.NoError(t, err)
		assert.NotNil(t, st)

		// Verify both named graphs are loaded
		ngBase := st.NamedGraph(ngBaseIRI)
		assert.NotNil(t, ngBase)

		ngImport := st.NamedGraph(ngImportIRI)
		assert.NotNil(t, ngImport, "Imported named graph should also be loaded")

		// Verify data in both graphs
		baseNode := ngBase.GetIRINodeByFragment("baseNode")
		assert.NotNil(t, baseNode)

		importNode := ngImport.GetIRINodeByFragment("importNode")
		assert.NotNil(t, importNode)

		// Verify the imports
		directImports := ngBase.DirectImports()
		assert.Equal(t, 1, len(directImports))
		assert.Equal(t, ngImportIRI, directImports[0].IRI())
	})
}

// Test_LocalFullRepository_CheckoutRevision_NonExistent tests error handling for non-existent revision
func Test_LocalFullRepository_CheckoutRevision_NonExistent(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a403").URN())

	repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
	assert.NoError(t, err)
	defer repo.Close()

	st := repo.OpenStage(sst.DefaultTriplexMode)
	ng := st.CreateNamedGraph(ngIRI)
	ng.CreateIRINode("node")

	_, _, err = st.Commit(context.TODO(), "First commit", sst.DefaultBranch)
	assert.NoError(t, err)

	ds, err := repo.Dataset(context.TODO(), ngIRI)
	assert.NoError(t, err)

	// Get a valid dataset revision first
	info := ng.Info()
	validRevision := info.DatasetRevision
	assert.False(t, validRevision.IsNil())

	// Try to checkout using a modified/non-existent revision (flip some bits)
	nonExistentRevision := validRevision
	nonExistentRevision[0] = ^nonExistentRevision[0]
	nonExistentRevision[1] = ^nonExistentRevision[1]

	_, err = ds.CheckoutRevision(context.TODO(), nonExistentRevision, sst.DefaultTriplexMode)
	assert.Error(t, err, "Should return error for non-existent revision")
}

// Test_RemoteRepository_CheckoutRevision_Basic tests CheckoutRevision via remote repository
func Test_RemoteRepository_CheckoutRevision_Basic(t *testing.T) {
	serverDir := filepath.Join(t.TempDir(), t.Name())
	ngIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a404").URN())

	var dsRevision1 sst.Hash

	// Setup server repository
	serverRepo, err := sst.CreateLocalRepository(serverDir, "default@semanticstep.net", "default", true)
	require.NoError(t, err)

	st := serverRepo.OpenStage(sst.DefaultTriplexMode)
	ng := st.CreateNamedGraph(ngIRI)
	node := ng.CreateIRINode("testNode")
	node.AddStatement(rdf.Type, rep.SchematicPort)

	_, _, err = st.Commit(context.TODO(), "First commit", sst.DefaultBranch)
	require.NoError(t, err)

	info := ng.Info()
	dsRevision1 = info.DatasetRevision
	require.False(t, dsRevision1.IsNil())
	serverRepo.Close()

	// Start server
	url := testutil.ServerServe(t, serverDir)
	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)

	constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

	// Connect as client
	clientRepo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
	require.NoError(t, err)
	defer clientRepo.Close()

	ds, err := clientRepo.Dataset(constructCtx, ngIRI)
	require.NoError(t, err)

	// Checkout by dataset revision via remote
	checkoutSt, err := ds.CheckoutRevision(constructCtx, dsRevision1, sst.DefaultTriplexMode)
	require.NoError(t, err)
	require.NotNil(t, checkoutSt)

	// Verify data
	ngCheckedOut := checkoutSt.NamedGraph(ngIRI)
	require.NotNil(t, ngCheckedOut)

	checkedOutNode := ngCheckedOut.GetIRINodeByFragment("testNode")
	require.NotNil(t, checkedOutNode)

	// Verify checkout info
	checkedOutInfo := ngCheckedOut.Info()
	assert.Equal(t, dsRevision1, checkedOutInfo.DatasetRevision)
	// For CheckoutRevision from a committed revision, commits should be populated
	// because the server sends DatasetRevisionCommitHash mapping
	assert.NotEmpty(t, checkedOutInfo.CheckedOutCommits)
}

// Test_RemoteRepository_CheckoutRevision_WithImports tests remote CheckoutRevision with imports
func Test_RemoteRepository_CheckoutRevision_WithImports(t *testing.T) {
	serverDir := filepath.Join(t.TempDir(), t.Name())
	ngBaseIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a405").URN())
	ngImportIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a406").URN())

	var baseDSRevision sst.Hash

	// Setup server repository with imports
	serverRepo, err := sst.CreateLocalRepository(serverDir, "default@semanticstep.net", "default", true)
	require.NoError(t, err)

	// Create import dataset
	st := serverRepo.OpenStage(sst.DefaultTriplexMode)
	ngImport := st.CreateNamedGraph(ngImportIRI)
	importNode := ngImport.CreateIRINode("importNode")
	importNode.AddStatement(rdf.Type, rep.SchematicPort)

	_, _, err = st.Commit(context.TODO(), "Import commit", sst.DefaultBranch)
	require.NoError(t, err)

	// Create base dataset with import (use same stage)
	ngBase := st.CreateNamedGraph(ngBaseIRI)
	baseNode := ngBase.CreateIRINode("baseNode")
	baseNode.AddStatement(rdf.Type, rep.SchematicPort)

	err = ngBase.AddImport(ngImport)
	require.NoError(t, err)

	_, _, err = st.Commit(context.TODO(), "Base with import", sst.DefaultBranch)
	require.NoError(t, err)

	info := ngBase.Info()
	baseDSRevision = info.DatasetRevision
	serverRepo.Close()

	// Start server
	url := testutil.ServerServe(t, serverDir)
	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)

	constructCtx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

	// Connect as client
	clientRepo, err := sst.OpenRemoteRepository(constructCtx, url, transportCreds)
	require.NoError(t, err)
	defer clientRepo.Close()

	ds, err := clientRepo.Dataset(constructCtx, ngBaseIRI)
	require.NoError(t, err)

	checkoutSt, err := ds.CheckoutRevision(constructCtx, baseDSRevision, sst.DefaultTriplexMode)
	require.NoError(t, err)
	require.NotNil(t, checkoutSt)

	// Verify both graphs loaded
	ngBaseCheckedOut := checkoutSt.NamedGraph(ngBaseIRI)
	require.NotNil(t, ngBaseCheckedOut)

	ngImportCheckedOut := checkoutSt.NamedGraph(ngImportIRI)
	require.NotNil(t, ngImportCheckedOut)

	// Verify import relationship
	directImports := ngBaseCheckedOut.DirectImports()
	assert.Equal(t, 1, len(directImports))
}

// Test_LocalFullRepository_CheckoutRevision_MultipleCommits tests CheckoutRevision with different commits
func Test_LocalFullRepository_CheckoutRevision_MultipleCommits(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a408").URN())

	var dsRevision1, dsRevision2, dsRevision3 sst.Hash

	t.Run("create_multiple_commits", func(t *testing.T) {

		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
		assert.NoError(t, err)
		defer repo.Close()

		st := repo.OpenStage(sst.DefaultTriplexMode)
		ng := st.CreateNamedGraph(ngIRI)

		// First commit
		node1 := ng.CreateIRINode("node1")
		node1.AddStatement(rdf.Type, rep.SchematicPort)
		_, _, err = st.Commit(context.TODO(), "First commit", sst.DefaultBranch)
		assert.NoError(t, err)
		info1 := ng.Info()
		dsRevision1 = info1.DatasetRevision

		// Second commit
		node2 := ng.CreateIRINode("node2")
		node2.AddStatement(rdf.Type, rep.SchematicPort)
		_, _, err = st.Commit(context.TODO(), "Second commit", sst.DefaultBranch)
		assert.NoError(t, err)
		info2 := ng.Info()
		dsRevision2 = info2.DatasetRevision

		// Third commit
		node3 := ng.CreateIRINode("node3")
		node3.AddStatement(rdf.Type, rep.Angle)
		_, _, err = st.Commit(context.TODO(), "Third commit", sst.DefaultBranch)
		assert.NoError(t, err)
		info3 := ng.Info()
		dsRevision3 = info3.DatasetRevision

		// Verify revisions are different
		assert.NotEqual(t, dsRevision1, dsRevision2)
		assert.NotEqual(t, dsRevision2, dsRevision3)
	})

	t.Run("checkout_each_revision", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		assert.NoError(t, err)
		defer repo.Close()

		ds, err := repo.Dataset(context.TODO(), ngIRI)
		assert.NoError(t, err)

		// Checkout first revision - should only have node1
		st1, err := ds.CheckoutRevision(context.TODO(), dsRevision1, sst.DefaultTriplexMode)
		assert.NoError(t, err)
		ng1 := st1.NamedGraph(ngIRI)
		assert.NotNil(t, ng1.GetIRINodeByFragment("node1"))
		assert.Nil(t, ng1.GetIRINodeByFragment("node2"))
		assert.Nil(t, ng1.GetIRINodeByFragment("node3"))

		// Checkout second revision - should have node1 and node2
		st2, err := ds.CheckoutRevision(context.TODO(), dsRevision2, sst.DefaultTriplexMode)
		assert.NoError(t, err)
		ng2 := st2.NamedGraph(ngIRI)
		assert.NotNil(t, ng2.GetIRINodeByFragment("node1"))
		assert.NotNil(t, ng2.GetIRINodeByFragment("node2"))
		assert.Nil(t, ng2.GetIRINodeByFragment("node3"))

		// Checkout third revision - should have all nodes
		st3, err := ds.CheckoutRevision(context.TODO(), dsRevision3, sst.DefaultTriplexMode)
		assert.NoError(t, err)
		ng3 := st3.NamedGraph(ngIRI)
		assert.NotNil(t, ng3.GetIRINodeByFragment("node1"))
		assert.NotNil(t, ng3.GetIRINodeByFragment("node2"))
		assert.NotNil(t, ng3.GetIRINodeByFragment("node3"))

		// Verify dataset revisions are correct
		info1 := ng1.Info()
		assert.Equal(t, dsRevision1, info1.DatasetRevision)

		info2 := ng2.Info()
		assert.Equal(t, dsRevision2, info2.DatasetRevision)

		info3 := ng3.Info()
		assert.Equal(t, dsRevision3, info3.DatasetRevision)
	})
}

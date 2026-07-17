// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst_test

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/sst_test/testutil"
	"github.com/semanticstep/sst-core/sstauth"
	_ "github.com/semanticstep/sst-core/vocabularies/dict"
	"github.com/semanticstep/sst-core/vocabularies/rep"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func openLocalRepo(t *testing.T) (sst.Repository, context.Context) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), t.Name())
	repo, err := sst.CreateLocalRepository(dir, "test@example.com", "test", true)
	require.NoError(t, err)
	t.Cleanup(func() { repo.Close() })
	return repo, context.TODO()
}

func openRemoteRepo(t *testing.T) (sst.Repository, context.Context) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), t.Name())
	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(dir) })

	url := testutil.ServerServe(t, dir)
	ctx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)
	repo, err := sst.OpenRemoteRepository(ctx, url, transportCreds)
	require.NoError(t, err)
	t.Cleanup(func() { repo.Close() })
	return repo, ctx
}

func testNewDataset(t *testing.T, repo sst.Repository, ctx context.Context) {
	t.Helper()
	ngIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a500").URN())

	st := sst.OpenStage(sst.DefaultTriplexMode)
	ng := st.CreateNamedGraph(ngIRI)
	ng.CreateIRINode("node", rep.SchematicPort)

	err := st.LinkToRepository(ctx, repo, sst.DefaultBranch)
	require.NoError(t, err)

	info := ng.Info()
	assert.Empty(t, info.CheckedOutCommits, "new dataset should have no parent commits")
	assert.Empty(t, info.CheckedOutNGRevisions)
	assert.Empty(t, info.CheckedOutDSRevisions)

	commit, _, err := st.Commit(ctx, "initial import", sst.DefaultBranch)
	require.NoError(t, err)
	assert.NotEqual(t, sst.Hash{}, commit)

	ds, err := repo.Dataset(ctx, ng.IRI())
	require.NoError(t, err)
	branches, err := ds.Branches(ctx)
	require.NoError(t, err)
	assert.Contains(t, branches, sst.DefaultBranch)
	assert.Equal(t, commit, branches[sst.DefaultBranch])
}

func testExistingDatasetExistingBranch(t *testing.T, repo sst.Repository, ctx context.Context) {
	t.Helper()
	ngIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a501").URN())

	// First commit
	st1 := repo.OpenStage(sst.DefaultTriplexMode)
	ng1 := st1.CreateNamedGraph(ngIRI)
	ng1.CreateIRINode("node", rep.SchematicPort)
	firstCommit, _, err := st1.Commit(ctx, "first", sst.DefaultBranch)
	require.NoError(t, err)

	// Re-import modified content and link to repository
	st2 := sst.OpenStage(sst.DefaultTriplexMode)
	ng2 := st2.CreateNamedGraph(ngIRI)
	ng2.CreateIRINode("node", rep.SchematicPort)
	ng2.CreateIRINode("newNode", rep.SchematicPort)

	err = st2.LinkToRepository(ctx, repo, sst.DefaultBranch)
	require.NoError(t, err)

	info := ng2.Info()
	assert.Equal(t, []sst.Hash{firstCommit}, info.CheckedOutCommits)
	assert.Len(t, info.CheckedOutNGRevisions, 1)
	assert.NotEqual(t, sst.Hash{}, info.CheckedOutNGRevisions[0])
	assert.Len(t, info.CheckedOutDSRevisions, 1)
	assert.NotEqual(t, sst.Hash{}, info.CheckedOutDSRevisions[0])

	secondCommit, _, err := st2.Commit(ctx, "second", sst.DefaultBranch)
	require.NoError(t, err)
	assert.NotEqual(t, firstCommit, secondCommit)

	ds, err := repo.Dataset(ctx, ngIRI)
	require.NoError(t, err)
	cd, err := ds.CommitDetailsByBranch(ctx, sst.DefaultBranch)
	require.NoError(t, err)
	assert.Equal(t, secondCommit, cd.Commit)
	assert.Contains(t, cd.ParentCommits[ngIRI], firstCommit)
}

func testExistingDatasetNewBranch(t *testing.T, repo sst.Repository, ctx context.Context) {
	t.Helper()
	ngIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a502").URN())

	st1 := repo.OpenStage(sst.DefaultTriplexMode)
	ng1 := st1.CreateNamedGraph(ngIRI)
	ng1.CreateIRINode("node", rep.SchematicPort)
	_, _, err := st1.Commit(ctx, "first", sst.DefaultBranch)
	require.NoError(t, err)

	st2 := sst.OpenStage(sst.DefaultTriplexMode)
	ng2 := st2.CreateNamedGraph(ngIRI)
	ng2.CreateIRINode("node", rep.SchematicPort)
	ng2.CreateIRINode("featureNode", rep.SchematicPort)

	err = st2.LinkToRepository(ctx, repo, "feature")
	require.NoError(t, err)

	info := ng2.Info()
	assert.Empty(t, info.CheckedOutCommits, "new branch should have no parent history")
	assert.Empty(t, info.CheckedOutNGRevisions)
	assert.Empty(t, info.CheckedOutDSRevisions)

	featureCommit, _, err := st2.Commit(ctx, "feature commit", "feature")
	require.NoError(t, err)

	ds, err := repo.Dataset(ctx, ngIRI)
	require.NoError(t, err)
	branches, err := ds.Branches(ctx)
	require.NoError(t, err)
	assert.Contains(t, branches, sst.DefaultBranch)
	assert.Contains(t, branches, "feature")
	assert.Equal(t, featureCommit, branches["feature"])

	cd, err := ds.CommitDetailsByBranch(ctx, "feature")
	require.NoError(t, err)
	assert.Empty(t, cd.ParentCommits[ngIRI], "new branch commit should have no parents")
}

func testMultipleNamedGraphs(t *testing.T, repo sst.Repository, ctx context.Context) {
	t.Helper()
	existingIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a503").URN())
	newIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a504").URN())

	// Pre-populate one dataset
	st1 := repo.OpenStage(sst.DefaultTriplexMode)
	ng1 := st1.CreateNamedGraph(existingIRI)
	ng1.CreateIRINode("node", rep.SchematicPort)
	_, _, err := st1.Commit(ctx, "first", sst.DefaultBranch)
	require.NoError(t, err)

	// Import stage with one existing and one new graph
	st2 := sst.OpenStage(sst.DefaultTriplexMode)
	ngExisting := st2.CreateNamedGraph(existingIRI)
	ngExisting.CreateIRINode("node", rep.SchematicPort)
	ngNew := st2.CreateNamedGraph(newIRI)
	ngNew.CreateIRINode("node", rep.SchematicPort)

	err = st2.LinkToRepository(ctx, repo, sst.DefaultBranch)
	require.NoError(t, err)

	existingInfo := ngExisting.Info()
	assert.Len(t, existingInfo.CheckedOutCommits, 1, "existing graph should inherit history")
	assert.NotEqual(t, sst.Hash{}, existingInfo.CheckedOutNGRevisions[0])

	newInfo := ngNew.Info()
	assert.Empty(t, newInfo.CheckedOutCommits, "new graph should have no parent history")

	_, _, err = st2.Commit(ctx, "multi-ng commit", sst.DefaultBranch)
	require.NoError(t, err)

	_, err = repo.Dataset(ctx, existingIRI)
	require.NoError(t, err)
	_, err = repo.Dataset(ctx, newIRI)
	require.NoError(t, err)
}

func testRdfReadRoundTrip(t *testing.T, repo sst.Repository, ctx context.Context) {
	t.Helper()
	ngIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a505").URN())

	// First commit
	st1 := repo.OpenStage(sst.DefaultTriplexMode)
	ng1 := st1.CreateNamedGraph(ngIRI)
	ng1.CreateIRINode("node", rep.SchematicPort)
	_, _, err := st1.Commit(ctx, "first", sst.DefaultBranch)
	require.NoError(t, err)

	// Export as Turtle
	var buf bytes.Buffer
	err = ng1.RdfWrite(&buf, sst.RdfFormatTurtle)
	require.NoError(t, err)

	// Re-import via RdfRead (same content; the test verifies history inheritance)
	st2, err := sst.RdfRead(bufio.NewReader(bytes.NewReader(buf.Bytes())), sst.RdfFormatTurtle, sst.StrictHandler, sst.DefaultTriplexMode)
	require.NoError(t, err)

	ng2 := st2.NamedGraph(ngIRI)
	require.NotNil(t, ng2)
	assert.Nil(t, st2.Repository(), "RdfRead stage should not be linked to a repository")

	err = st2.LinkToRepository(ctx, repo, sst.DefaultBranch)
	require.NoError(t, err)
	assert.Equal(t, repo, st2.Repository())

	info := ng2.Info()
	assert.Len(t, info.CheckedOutCommits, 1, "imported graph should inherit history")

	_, _, err = st2.Commit(ctx, "imported update", sst.DefaultBranch)
	require.NoError(t, err)
}

func Test_LinkToRepository_NewDataset(t *testing.T) {
	repo, ctx := openLocalRepo(t)
	testNewDataset(t, repo, ctx)
}

func Test_LinkToRepository_ExistingDatasetExistingBranch(t *testing.T) {
	repo, ctx := openLocalRepo(t)
	testExistingDatasetExistingBranch(t, repo, ctx)
}

func Test_LinkToRepository_ExistingDatasetNewBranch(t *testing.T) {
	repo, ctx := openLocalRepo(t)
	testExistingDatasetNewBranch(t, repo, ctx)
}

func Test_LinkToRepository_MultipleNamedGraphs(t *testing.T) {
	repo, ctx := openLocalRepo(t)
	testMultipleNamedGraphs(t, repo, ctx)
}

func Test_LinkToRepository_RdfReadRoundTrip(t *testing.T) {
	repo, ctx := openLocalRepo(t)
	testRdfReadRoundTrip(t, repo, ctx)
}

func Test_LinkToRepository_Remote_NewDataset(t *testing.T) {
	repo, ctx := openRemoteRepo(t)
	testNewDataset(t, repo, ctx)
}

func Test_LinkToRepository_Remote_ExistingDatasetExistingBranch(t *testing.T) {
	repo, ctx := openRemoteRepo(t)
	testExistingDatasetExistingBranch(t, repo, ctx)
}

func Test_LinkToRepository_Remote_ExistingDatasetNewBranch(t *testing.T) {
	repo, ctx := openRemoteRepo(t)
	testExistingDatasetNewBranch(t, repo, ctx)
}

func Test_LinkToRepository_Remote_MultipleNamedGraphs(t *testing.T) {
	repo, ctx := openRemoteRepo(t)
	testMultipleNamedGraphs(t, repo, ctx)
}

func Test_LinkToRepository_Remote_RdfReadRoundTrip(t *testing.T) {
	repo, ctx := openRemoteRepo(t)
	testRdfReadRoundTrip(t, repo, ctx)
}

func Test_LinkToRepository_EmptyBranch(t *testing.T) {
	repo, ctx := openLocalRepo(t)

	ngIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a506").URN())
	st := sst.OpenStage(sst.DefaultTriplexMode)
	ng := st.CreateNamedGraph(ngIRI)
	ng.CreateIRINode("node", rep.SchematicPort)

	err := st.LinkToRepository(ctx, repo, "")
	assert.ErrorIs(t, err, sst.ErrEmptyBranchName)
}

func Test_LinkToRepository_NilRepository(t *testing.T) {
	ngIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a507").URN())
	st := sst.OpenStage(sst.DefaultTriplexMode)
	ng := st.CreateNamedGraph(ngIRI)
	ng.CreateIRINode("node", rep.SchematicPort)

	err := st.LinkToRepository(context.TODO(), nil, sst.DefaultBranch)
	assert.ErrorIs(t, err, sst.ErrRepositoryNotFound)
}

func Test_LinkToRepository_StageLinkedToDifferentRepository(t *testing.T) {
	dir1 := filepath.Join(t.TempDir(), t.Name()+"1")
	repo1, err := sst.CreateLocalRepository(dir1, "test@example.com", "test", true)
	require.NoError(t, err)
	defer repo1.Close()

	dir2 := filepath.Join(t.TempDir(), t.Name()+"2")
	repo2, err := sst.CreateLocalRepository(dir2, "test@example.com", "test", true)
	require.NoError(t, err)
	defer repo2.Close()

	ngIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a508").URN())

	// Create a dataset in repo1 so that we can check out a branch from it.
	st1 := repo1.OpenStage(sst.DefaultTriplexMode)
	ng1 := st1.CreateNamedGraph(ngIRI)
	ng1.CreateIRINode("node", rep.SchematicPort)
	_, _, err = st1.Commit(context.TODO(), "first", sst.DefaultBranch)
	require.NoError(t, err)

	// Simulate a stage already linked to repo1 by checking out a branch.
	ds, err := repo1.Dataset(context.TODO(), ngIRI)
	require.NoError(t, err)
	st, err := ds.CheckoutBranch(context.TODO(), sst.DefaultBranch, sst.DefaultTriplexMode)
	require.NoError(t, err)

	err = st.LinkToRepository(context.TODO(), repo2, sst.DefaultBranch)
	assert.ErrorIs(t, err, sst.ErrStagesRepositoryMismatch)
}

func Test_LinkToRepository_AlreadyLinkedNamedGraph(t *testing.T) {
	repo, ctx := openLocalRepo(t)

	ngIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a509").URN())

	st := repo.OpenStage(sst.DefaultTriplexMode)
	ng := st.CreateNamedGraph(ngIRI)
	ng.CreateIRINode("node", rep.SchematicPort)
	_, _, err := st.Commit(ctx, "first", sst.DefaultBranch)
	require.NoError(t, err)

	// After commit, the NamedGraph carries checkout metadata.
	err = ng.LinkToRepository(ctx, repo, sst.DefaultBranch)
	assert.ErrorIs(t, err, sst.ErrNamedGraphAlreadyLinked)
}

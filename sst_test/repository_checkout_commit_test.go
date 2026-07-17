// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/sst_test/testutil"
	"github.com/semanticstep/sst-core/sstauth"
	"github.com/semanticstep/sst-core/vocabularies/rdf"
	"github.com/semanticstep/sst-core/vocabularies/rep"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func stageNamedGraphIRIs(st sst.Stage) map[sst.IRI]struct{} {
	m := make(map[sst.IRI]struct{})
	for _, ng := range st.NamedGraphs() {
		m[ng.IRI()] = struct{}{}
	}
	return m
}

func commitDetailsNGIRIs(cd *sst.CommitDetails) map[sst.IRI]struct{} {
	m := make(map[sst.IRI]struct{}, len(cd.NamedGraphRevisions))
	for iri := range cd.NamedGraphRevisions {
		m[iri] = struct{}{}
	}
	return m
}

func assertStageMatchesCommitNGs(t *testing.T, st sst.Stage, cd *sst.CommitDetails) {
	t.Helper()
	assert.Equal(t, commitDetailsNGIRIs(cd), stageNamedGraphIRIs(st))
	for ngIRI, wantNGR := range cd.NamedGraphRevisions {
		ng := st.NamedGraph(ngIRI)
		require.NotNil(t, ng, "missing NG %s", ngIRI)
		info := ng.Info()
		assert.Equal(t, wantNGR, info.NamedGraphRevision, "NGR mismatch for %s", ngIRI)
	}
}

func Test_LocalFullRepository_CheckoutCommit_Basic(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a500").URN())

	var commitHash sst.Hash

	t.Run("create_and_commit", func(t *testing.T) {
		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
		require.NoError(t, err)
		defer repo.Close()

		st := repo.OpenStage(sst.DefaultTriplexMode)
		ng := st.CreateNamedGraph(ngIRI)
		mainNode := ng.CreateIRINode("mainNode")
		mainNode.AddStatement(rdf.Type, rep.SchematicPort)

		commitHash, _, err = st.Commit(context.TODO(), "First commit", sst.DefaultBranch)
		require.NoError(t, err)
		require.False(t, commitHash.IsNil())
	})

	t.Run("checkout_commit", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		require.NoError(t, err)
		defer repo.Close()

		cds, err := repo.CommitDetails(context.TODO(), []sst.Hash{commitHash})
		require.NoError(t, err)
		require.Len(t, cds, 1)

		st, err := repo.CheckoutCommit(context.TODO(), commitHash, sst.DefaultTriplexMode)
		require.NoError(t, err)
		require.NotNil(t, st)

		assertStageMatchesCommitNGs(t, st, cds[0])

		ng := st.NamedGraph(ngIRI)
		require.NotNil(t, ng)
		mainNode := ng.GetIRINodeByFragment("mainNode")
		require.NotNil(t, mainNode)
	})
}

func Test_LocalFullRepository_CheckoutCommit_WithImports(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngBaseIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a501").URN())
	ngImportIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a502").URN())

	var commitHash sst.Hash

	t.Run("setup_imports_and_commit", func(t *testing.T) {
		repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
		require.NoError(t, err)
		defer repo.Close()

		st := repo.OpenStage(sst.DefaultTriplexMode)
		ngImport := st.CreateNamedGraph(ngImportIRI)
		importNode := ngImport.CreateIRINode("importNode")
		importNode.AddStatement(rdf.Type, rep.SchematicPort)

		_, _, err = st.Commit(context.TODO(), "Import dataset commit", sst.DefaultBranch)
		require.NoError(t, err)

		ngBase := st.CreateNamedGraph(ngBaseIRI)
		baseNode := ngBase.CreateIRINode("baseNode")
		baseNode.AddStatement(rdf.Type, rep.SchematicPort)
		require.NoError(t, ngBase.AddImport(ngImport))

		commitHash, _, err = st.Commit(context.TODO(), "Base dataset with import", sst.DefaultBranch)
		require.NoError(t, err)
	})

	t.Run("checkout_commit", func(t *testing.T) {
		repo, err := sst.OpenLocalRepository(dir, "default@semanticstep.net", "default")
		require.NoError(t, err)
		defer repo.Close()

		cds, err := repo.CommitDetails(context.TODO(), []sst.Hash{commitHash})
		require.NoError(t, err)
		require.Len(t, cds, 1)

		st, err := repo.CheckoutCommit(context.TODO(), commitHash, sst.DefaultTriplexMode)
		require.NoError(t, err)

		assertStageMatchesCommitNGs(t, st, cds[0])

		ngBase := st.NamedGraph(ngBaseIRI)
		require.NotNil(t, ngBase)
		ngImport := st.NamedGraph(ngImportIRI)
		require.NotNil(t, ngImport)
		require.NotNil(t, ngBase.GetIRINodeByFragment("baseNode"))
		require.NotNil(t, ngImport.GetIRINodeByFragment("importNode"))
	})
}

func Test_LocalFullRepository_CheckoutCommit_NotFound(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	ngIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a504").URN())

	repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", true)
	require.NoError(t, err)
	defer repo.Close()

	st := repo.OpenStage(sst.DefaultTriplexMode)
	ng := st.CreateNamedGraph(ngIRI)
	ng.CreateIRINode("node")
	commitHash, _, err := st.Commit(context.TODO(), "First commit", sst.DefaultBranch)
	require.NoError(t, err)

	missing := commitHash
	missing[0] = ^missing[0]

	_, err = repo.CheckoutCommit(context.TODO(), missing, sst.DefaultTriplexMode)
	require.Error(t, err)
	require.ErrorIs(t, err, sst.ErrCommitNotFound)
}

func Test_LocalBasicRepository_CheckoutCommit_NotSupported(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())

	repo, err := sst.CreateLocalRepository(dir, "default@semanticstep.net", "default", false)
	require.NoError(t, err)
	defer repo.Close()

	_, err = repo.CheckoutCommit(context.TODO(), sst.Hash{1}, sst.DefaultTriplexMode)
	require.Error(t, err)
	require.True(t, errors.Is(err, sst.ErrNotSupported))
}

func Test_RemoteRepository_CheckoutCommit_Basic(t *testing.T) {
	serverDir := filepath.Join(t.TempDir(), t.Name())
	ngIRI := sst.IRI(uuid.MustParse("c1efcf54-3e8e-4cc7-a7d1-82a9f613a503").URN())

	var commitHash sst.Hash

	serverRepo, err := sst.CreateLocalRepository(serverDir, "default@semanticstep.net", "default", true)
	require.NoError(t, err)

	st := serverRepo.OpenStage(sst.DefaultTriplexMode)
	ng := st.CreateNamedGraph(ngIRI)
	node := ng.CreateIRINode("testNode")
	node.AddStatement(rdf.Type, rep.SchematicPort)

	commitHash, _, err = st.Commit(context.TODO(), "First commit", sst.DefaultBranch)
	require.NoError(t, err)
	serverRepo.Close()

	url := testutil.ServerServe(t, serverDir)
	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)

	ctx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

	clientRepo, err := sst.OpenRemoteRepository(ctx, url, transportCreds)
	require.NoError(t, err)
	defer clientRepo.Close()

	cds, err := clientRepo.CommitDetails(ctx, []sst.Hash{commitHash})
	require.NoError(t, err)
	require.Len(t, cds, 1)

	checkoutSt, err := clientRepo.CheckoutCommit(ctx, commitHash, sst.DefaultTriplexMode)
	require.NoError(t, err)

	assertStageMatchesCommitNGs(t, checkoutSt, cds[0])
	require.NotNil(t, checkoutSt.NamedGraph(ngIRI))
}

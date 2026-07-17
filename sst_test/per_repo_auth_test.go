// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/sst_test/testutil"
	"github.com/semanticstep/sst-core/sstauth"
	_ "github.com/semanticstep/sst-core/vocabularies/dict"
	"github.com/semanticstep/sst-core/vocabularies/lci"
	"github.com/blevesearch/bleve/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Test_PerRepoAuth_AccessControl verifies that per-sub-repository access
// control isolates repositories using per-repo Keycloak client audiences.
func Test_PerRepoAuth_AccessControl(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	clientID := "grpc://test.super.repo"
	serverURL, provider := testutil.SuperServerServeWithPerRepoAuth(t, dir, clientID)

	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)

	ctx := context.Background()

	// Use a SuperAdmin token to create and seed sub-repositories.
	superToken := provider.IssueToken(
		"admin@test",
		[]string{clientID},
		map[string][]string{clientID: {"SuperAdmin"}},
	)
	superCtx := sstauth.ContextWithAuthProvider(ctx, &testutil.TestProvider{RawToken: superToken})

	superRepo, err := sst.OpenRemoteSuperRepository(superCtx, serverURL, transportCreds)
	require.NoError(t, err)
	defer superRepo.Close()

	repoA, err := superRepo.Create(superCtx, "repoA")
	require.NoError(t, err)
	repoB, err := superRepo.Create(superCtx, "repoB")
	require.NoError(t, err)

	// Seed each repo with a per-repo ReadWrite token, because repository
	// operations require the per-repo client audience.
	repoAToken := provider.IssueToken(
		"writer@test",
		[]string{clientID + "#repoA"},
		map[string][]string{clientID + "#repoA": {"ReadWrite"}},
	)
	repoACtx := sstauth.ContextWithAuthProvider(ctx, &testutil.TestProvider{RawToken: repoAToken})
	repoBToken := provider.IssueToken(
		"writer@test",
		[]string{clientID + "#repoB"},
		map[string][]string{clientID + "#repoB": {"ReadWrite"}},
	)
	repoBCtx := sstauth.ContextWithAuthProvider(ctx, &testutil.TestProvider{RawToken: repoBToken})

	seedRepo(t, repoACtx, repoA, "repoA-data")
	seedRepo(t, repoBCtx, repoB, "repoB-data")

	t.Run("super admin can list repos", func(t *testing.T) {
		names, err := superRepo.List(superCtx)
		require.NoError(t, err)
		assert.Subset(t, names, []string{"default", "repoA", "repoB"})
	})

	t.Run("super token without admin role cannot manage repos", func(t *testing.T) {
		roToken := provider.IssueToken(
			"user@test",
			[]string{clientID},
			map[string][]string{clientID: {"ReadOnly"}},
		)
		roCtx := sstauth.ContextWithAuthProvider(ctx, &testutil.TestProvider{RawToken: roToken})
		_, err := superRepo.List(roCtx)
		require.Error(t, err)
		assert.Equal(t, codes.PermissionDenied, status.Code(err))
	})

	t.Run("per-repo ReadOnly can read own repo", func(t *testing.T) {
		token := provider.IssueToken(
			"user@test",
			[]string{clientID + "#repoA"},
			map[string][]string{clientID + "#repoA": {"ReadOnly"}},
		)
		userCtx := sstauth.ContextWithAuthProvider(ctx, &testutil.TestProvider{RawToken: token})

		r, err := superRepo.Get(userCtx, "repoA")
		require.NoError(t, err)
		_, err = r.Datasets(userCtx)
		require.NoError(t, err)
	})

	t.Run("per-repo ReadOnly cannot read other repo", func(t *testing.T) {
		token := provider.IssueToken(
			"user@test",
			[]string{clientID + "#repoA"},
			map[string][]string{clientID + "#repoA": {"ReadOnly"}},
		)
		userCtx := sstauth.ContextWithAuthProvider(ctx, &testutil.TestProvider{RawToken: token})

		r, err := superRepo.Get(userCtx, "repoB")
		require.NoError(t, err)
		_, err = r.Datasets(userCtx)
		require.Error(t, err)
		assert.Equal(t, codes.PermissionDenied, status.Code(err))
	})

	t.Run("per-repo ReadOnly can search own repo", func(t *testing.T) {
		token := provider.IssueToken(
			"user@test",
			[]string{clientID + "#repoA"},
			map[string][]string{clientID + "#repoA": {"ReadOnly"}},
		)
		userCtx := sstauth.ContextWithAuthProvider(ctx, &testutil.TestProvider{RawToken: token})

		r, err := superRepo.Get(userCtx, "repoA")
		require.NoError(t, err)
		res, err := r.Bleve().SearchInContext(userCtx, bleve.NewSearchRequest(bleve.NewMatchAllQuery()))
		require.NoError(t, err)
		assert.Equal(t, 1, int(res.Total))
	})

	t.Run("per-repo ReadOnly cannot search other repo", func(t *testing.T) {
		token := provider.IssueToken(
			"user@test",
			[]string{clientID + "#repoA"},
			map[string][]string{clientID + "#repoA": {"ReadOnly"}},
		)
		userCtx := sstauth.ContextWithAuthProvider(ctx, &testutil.TestProvider{RawToken: token})

		r, err := superRepo.Get(userCtx, "repoB")
		require.NoError(t, err)
		_, err = r.Bleve().SearchInContext(userCtx, bleve.NewSearchRequest(bleve.NewMatchAllQuery()))
		require.Error(t, err)
		assert.Equal(t, codes.PermissionDenied, status.Code(err))
	})

	t.Run("per-repo ReadOnly cannot write own repo", func(t *testing.T) {
		token := provider.IssueToken(
			"user@test",
			[]string{clientID + "#repoA"},
			map[string][]string{clientID + "#repoA": {"ReadOnly"}},
		)
		userCtx := sstauth.ContextWithAuthProvider(ctx, &testutil.TestProvider{RawToken: token})

		r, err := superRepo.Get(userCtx, "repoA")
		require.NoError(t, err)
		err = writeRepo(t, userCtx, r, "extra")
		require.Error(t, err)
		assert.Equal(t, codes.PermissionDenied, status.Code(err))
	})

	t.Run("per-repo ReadWrite can write own repo", func(t *testing.T) {
		token := provider.IssueToken(
			"user@test",
			[]string{clientID + "#repoA"},
			map[string][]string{clientID + "#repoA": {"ReadWrite"}},
		)
		userCtx := sstauth.ContextWithAuthProvider(ctx, &testutil.TestProvider{RawToken: token})

		r, err := superRepo.Get(userCtx, "repoA")
		require.NoError(t, err)
		err = writeRepo(t, userCtx, r, "new-data")
		require.NoError(t, err)
	})

	t.Run("per-repo ReadWrite cannot write other repo", func(t *testing.T) {
		token := provider.IssueToken(
			"user@test",
			[]string{clientID + "#repoA"},
			map[string][]string{clientID + "#repoA": {"ReadWrite"}},
		)
		userCtx := sstauth.ContextWithAuthProvider(ctx, &testutil.TestProvider{RawToken: token})

		r, err := superRepo.Get(userCtx, "repoB")
		require.NoError(t, err)
		err = writeRepo(t, userCtx, r, "intrusion")
		require.Error(t, err)
		assert.Equal(t, codes.PermissionDenied, status.Code(err))
	})
}

// Test_PerRepoAuthBackwardCompatibility verifies that a single RepositoryServer
// with per-repository authorization disabled still accepts tokens that carry
// the super client ID audience, preserving the original authorization model.
func Test_PerRepoAuthBackwardCompatibility(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	clientID := "grpc://test.single.repo"
	serverURL, provider := testutil.ServerServeWithOIDC(t, dir, clientID)

	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)

	ctx := context.Background()

	rwToken := provider.IssueToken(
		"admin@test",
		[]string{clientID},
		map[string][]string{clientID: {"ReadWrite"}},
	)
	rwCtx := sstauth.ContextWithAuthProvider(ctx, &testutil.TestProvider{RawToken: rwToken})

	repo, err := sst.OpenRemoteRepository(rwCtx, serverURL, transportCreds)
	require.NoError(t, err)
	defer repo.Close()

	seedRepo(t, rwCtx, repo, "initial")

	t.Run("super ReadWrite can read and write", func(t *testing.T) {
		require.NoError(t, writeRepo(t, rwCtx, repo, "more"))

		_, err := repo.Datasets(rwCtx)
		require.NoError(t, err)

		res, err := repo.Bleve().SearchInContext(rwCtx, bleve.NewSearchRequest(bleve.NewMatchAllQuery()))
		require.NoError(t, err)
		assert.Equal(t, 2, int(res.Total))
	})

	t.Run("super ReadOnly cannot write", func(t *testing.T) {
		roToken := provider.IssueToken(
			"user@test",
			[]string{clientID},
			map[string][]string{clientID: {"ReadOnly"}},
		)
		roCtx := sstauth.ContextWithAuthProvider(ctx, &testutil.TestProvider{RawToken: roToken})

		err := writeRepo(t, roCtx, repo, "forbidden")
		require.Error(t, err)
		assert.Equal(t, codes.PermissionDenied, status.Code(err))
	})

	t.Run("per-repo audience is rejected when PerRepoAuth disabled", func(t *testing.T) {
		perRepoToken := provider.IssueToken(
			"user@test",
			[]string{clientID + "#default"},
			map[string][]string{clientID + "#default": {"ReadWrite"}},
		)
		perRepoCtx := sstauth.ContextWithAuthProvider(ctx, &testutil.TestProvider{RawToken: perRepoToken})

		_, err := repo.Datasets(perRepoCtx)
		require.Error(t, err)
		assert.Equal(t, codes.PermissionDenied, status.Code(err))
	})
}

func seedRepo(t *testing.T, ctx context.Context, repo sst.Repository, fragment string) {
	t.Helper()
	st := repo.OpenStage(sst.DefaultTriplexMode)
	ng := st.CreateNamedGraph(sst.IRI("urn:uuid:" + uuid.New().String()))
	ng.CreateIRINode(fragment, lci.Person)
	_, _, err := st.Commit(ctx, "seed", sst.DefaultBranch)
	require.NoError(t, err)
}

func writeRepo(t *testing.T, ctx context.Context, repo sst.Repository, fragment string) error {
	t.Helper()
	st := repo.OpenStage(sst.DefaultTriplexMode)
	ng := st.CreateNamedGraph(sst.IRI("urn:uuid:" + uuid.New().String()))
	ng.CreateIRINode(fragment, lci.Person)
	_, _, err := st.Commit(ctx, "write", sst.DefaultBranch)
	return err
}

// Test_PerRepoAuth_Info_BleveInfo_UsesRepoName verifies that Repository.Info on a
// per-repo authenticated SuperRepository includes RepoName in the GetBleveInfo RPC.
// Before the fix the client sent an empty GetRepoBleveInfoRequest, so the server
// defaulted to the "default" Repository Entry and rejected tokens scoped to other
// Repository Entries with "invalid audience".
func Test_PerRepoAuth_Info_BleveInfo_UsesRepoName(t *testing.T) {
	dir := t.TempDir()
	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)

	const superClientID = "grpc://test.info.repo"
	url, provider := testutil.SuperServerServeWithPerRepoAuth(t, dir, superClientID)

	// Create repoA as super-admin.
	superAdminToken := provider.IssueToken(
		"admin@example.com",
		[]string{superClientID},
		map[string][]string{superClientID: {"SuperAdmin"}},
	)
	superAdminProvider := testutil.TestProvider{RawToken: superAdminToken}
	adminCtx := sstauth.ContextWithAuthProvider(context.TODO(), superAdminProvider)

	superRepo, err := sst.OpenRemoteSuperRepository(adminCtx, url, transportCreds)
	require.NoError(t, err)
	defer superRepo.Close()

	_, err = superRepo.Create(adminCtx, "repoA")
	require.NoError(t, err)

	// Access repoA with a token scoped only to the per-repo client ID.
	repoAClientID := sstauth.PerRepoClientID(superClientID, "repoA")
	perRepoToken := provider.IssueToken(
		"user@example.com",
		[]string{repoAClientID},
		map[string][]string{repoAClientID: {"Admin"}},
	)
	perRepoProvider := testutil.TestProvider{RawToken: perRepoToken}
	perRepoCtx := sstauth.ContextWithAuthProvider(context.TODO(), perRepoProvider)

	repo, err := sst.OpenRemoteSuperRepository(perRepoCtx, url, transportCreds)
	require.NoError(t, err)
	defer repo.Close()

	repoAClient, err := repo.Get(perRepoCtx, "repoA")
	require.NoError(t, err)

	info, err := repoAClient.Info(perRepoCtx, sst.DefaultBranch)
	require.NoError(t, err)
	require.True(t, info.IsRemote)
}

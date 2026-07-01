// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst_test

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/sst_test/testutil"
	"github.com/semanticstep/sst-core/sstauth"
	_ "github.com/semanticstep/sst-core/vocabularies/dict"
	"github.com/semanticstep/sst-core/vocabularies/lci"
	"github.com/semanticstep/sst-core/vocabularies/rdfs"
	"github.com/stretchr/testify/require"
)

// fakeJWT returns an unsigned JWT whose payload contains resource_access roles.
// The server's testAuthFunc accepts any token that is not "test-token-1" or "test-token-2"
// as a bearer token, and GetRepositoryInfo extracts roles from the JWT payload without
// verifying the signature.
func fakeJWT(roles []string) string {
	header := map[string]string{"alg": "none", "typ": "JWT"}
	payload := map[string]any{
		"email": "admin@semanticstep.net",
		"resource_access": map[string]any{
			"test-client-id": map[string]any{"roles": roles},
		},
	}
	h, _ := json.Marshal(header)
	p, _ := json.Marshal(payload)
	return base64.RawURLEncoding.EncodeToString(h) + "." +
		base64.RawURLEncoding.EncodeToString(p) + "."
}

func Test_RemoteSuperRepository_Quota_CRUD(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)

	url := testutil.SuperServerServe(t, dir)
	superAdminProvider := testutil.TestProvider{RawToken: fakeJWT([]string{"super-admin"})}
	ctx := sstauth.ContextWithAuthProvider(context.TODO(), superAdminProvider)

	super, err := sst.OpenRemoteSuperRepository(ctx, url, transportCreds)
	require.NoError(t, err)
	defer super.Close()

	// Set and read per-repo quota.
	err = super.SetQuota(ctx, "repoA", 10*1024*1024)
	require.NoError(t, err)

	q, err := super.GetQuota(ctx, "repoA")
	require.NoError(t, err)
	require.Equal(t, int64(10*1024*1024), q.MaxSizeBytes)
	require.GreaterOrEqual(t, q.ActualSizeBytes, int64(0))

	// Set and read total quota.
	err = super.SetTotalQuota(ctx, 100*1024*1024)
	require.NoError(t, err)

	tq, err := super.GetTotalQuota(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(100*1024*1024), tq.MaxSizeBytes)
	require.GreaterOrEqual(t, tq.ActualSizeBytes, int64(0))

	// Set and read max repo count.
	err = super.SetMaxRepositoryCount(ctx, 5)
	require.NoError(t, err)
	require.Equal(t, 5, super.GetMaxRepositoryCount(ctx))
}

func Test_RemoteSuperRepository_Create_RespectsCountQuota(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)

	url := testutil.SuperServerServe(t, dir)
	superAdminProvider := testutil.TestProvider{RawToken: fakeJWT([]string{"super-admin"})}
	ctx := sstauth.ContextWithAuthProvider(context.TODO(), superAdminProvider)

	super, err := sst.OpenRemoteSuperRepository(ctx, url, transportCreds)
	require.NoError(t, err)
	defer super.Close()

	// default already exists.
	err = super.SetMaxRepositoryCount(ctx, 2)
	require.NoError(t, err)

	_, err = super.Create(ctx, "repoA")
	require.NoError(t, err)

	_, err = super.Create(ctx, "repoB")
	require.Error(t, err)
	require.True(t, errors.Is(err, sst.ErrQuotaExceeded))
}

func Test_RemoteRepository_DocumentSet_Quota(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)

	url := testutil.SuperServerServe(t, dir)
	superAdminProvider := testutil.TestProvider{RawToken: fakeJWT([]string{"super-admin"})}
	ctx := sstauth.ContextWithAuthProvider(context.TODO(), superAdminProvider)

	super, err := sst.OpenRemoteSuperRepository(ctx, url, transportCreds)
	require.NoError(t, err)
	defer super.Close()

	repo, err := super.Create(ctx, "repoA")
	require.NoError(t, err)

	// Set a tight quota that the empty repo already exceeds.
	err = super.SetQuota(ctx, "repoA", 1024)
	require.NoError(t, err)

	_, err = repo.DocumentSet(ctx, "text/plain", bufio.NewReader(strings.NewReader("hello world")))
	require.Error(t, err)
	require.True(t, errors.Is(err, sst.ErrQuotaExceeded))

	// Relax quota and verify upload succeeds.
	err = super.SetQuota(ctx, "repoA", 10*1024*1024)
	require.NoError(t, err)

	_, err = repo.DocumentSet(ctx, "text/plain", bufio.NewReader(strings.NewReader("hello world")))
	require.NoError(t, err)
}

func Test_RemoteRepository_Commit_Quota(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)

	url := testutil.SuperServerServe(t, dir)
	superAdminProvider := testutil.TestProvider{RawToken: fakeJWT([]string{"super-admin"})}
	ctx := sstauth.ContextWithAuthProvider(context.TODO(), superAdminProvider)

	super, err := sst.OpenRemoteSuperRepository(ctx, url, transportCreds)
	require.NoError(t, err)
	defer super.Close()

	repo, err := super.Create(ctx, "repoA")
	require.NoError(t, err)

	// Tight quota so any commit exceeds it.
	err = super.SetQuota(ctx, "repoA", 1024)
	require.NoError(t, err)

	stage := repo.OpenStage(sst.DefaultTriplexMode)
	ng := stage.CreateNamedGraph(sst.IRI("urn:uuid:12345678-1234-1234-1234-123456789abc"))
	node := ng.CreateIRINode("test", lci.Individual)
	node.AddStatement(rdfs.Label, sst.String("hello"))

	_, _, err = stage.Commit(ctx, "test commit", sst.DefaultBranch)
	require.Error(t, err)
	require.True(t, errors.Is(err, sst.ErrQuotaExceeded))
}

func Test_RemoteRepository_Info_QuotaFields(t *testing.T) {
	dir := filepath.Join(t.TempDir(), t.Name())
	transportCreds, err := testutil.TestTransportCreds()
	require.NoError(t, err)

	url := testutil.SuperServerServe(t, dir)

	// Use a super-admin provider for SuperRepository setup.
	superAdminProvider := testutil.TestProvider{RawToken: fakeJWT([]string{"super-admin"})}
	adminSetupCtx := sstauth.ContextWithAuthProvider(context.TODO(), superAdminProvider)
	super, err := sst.OpenRemoteSuperRepository(adminSetupCtx, url, transportCreds)
	require.NoError(t, err)
	defer super.Close()

	repo, err := super.Create(adminSetupCtx, "repoA")
	require.NoError(t, err)

	err = super.SetQuota(adminSetupCtx, "repoA", 20*1024*1024)
	require.NoError(t, err)

	// Non-admin users should not see quota fields.
	t.Run("non-admin", func(t *testing.T) {
		ctx := sstauth.ContextWithAuthProvider(context.TODO(), testutil.TestProviderInstance)

		info, err := repo.Info(ctx, "")
		require.NoError(t, err)
		require.True(t, info.IsRemote)
		require.Equal(t, int64(0), info.ActualRepositorySize)
		require.Equal(t, int64(0), info.MaxRepositorySize)
	})

	// Admin users should see quota fields.
	t.Run("admin", func(t *testing.T) {
		adminProvider := testutil.TestProvider{RawToken: fakeJWT([]string{"admin"})}
		ctx := sstauth.ContextWithAuthProvider(context.TODO(), adminProvider)
		super, err := sst.OpenRemoteSuperRepository(ctx, url, transportCreds)
		require.NoError(t, err)
		defer super.Close()

		repo, err := super.Get(ctx, "repoA")
		require.NoError(t, err)

		info, err := repo.Info(ctx, "")
		require.NoError(t, err)
		require.True(t, info.IsRemote)
		require.Greater(t, info.ActualRepositorySize, int64(0))
		require.Equal(t, int64(20*1024*1024), info.MaxRepositorySize)
	})
}

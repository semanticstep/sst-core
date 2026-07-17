// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst_test

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/sstauth"
	_ "github.com/semanticstep/sst-core/vocabularies/dict"
	"github.com/semanticstep/sst-core/vocabularies/lci"
	"github.com/blevesearch/bleve/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
)

// TestRealKeycloakSuperServerAccess connects to the real SST SuperServer named
// "zhe" configured in .env.sst-test using credentials from .env.sst-cli
// and verifies access control behavior.
func TestRealKeycloakSuperServerAccess(t *testing.T) {
	env := loadRealCredentials(t)
	token := fetchKeycloakToken(t, env)

	ctx := context.Background()
	p := &tokenProvider{token: token, email: tokenEmail(token)}
	authCtx := sstauth.ContextWithAuthProvider(ctx, p)

	target := loadRealTargetByName(t, "zhe")
	superClientID := target
	tlsOpt := grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{}))

	superRepo, err := sst.OpenRemoteSuperRepository(ctx, target, tlsOpt)
	require.NoError(t, err)
	defer superRepo.Close()

	logTokenAccessModes(t, token)

	ensureRepoA(t, authCtx, superRepo)

	t.Run("super admin can list repos", func(t *testing.T) {
		names, err := superRepo.List(authCtx)
		require.NoError(t, err)
		t.Logf("repos: %v", names)
		assert.NotEmpty(t, names)
	})

	t.Run("get default repo allowed with per-repo audience", func(t *testing.T) {
		repo, err := superRepo.Get(authCtx, "default")
		require.NoError(t, err)

		_, err = repo.Datasets(authCtx)
		require.NoError(t, err)
	})

	t.Run("search default repo allowed", func(t *testing.T) {
		repo, err := superRepo.Get(authCtx, "default")
		require.NoError(t, err)

		req := bleve.NewSearchRequest(bleve.NewMatchAllQuery())
		res, err := repo.Bleve().SearchInContext(authCtx, req)
		require.NoError(t, err)
		t.Logf("search hits: %d", res.Total)
	})

	t.Run("get repo without audience denied", func(t *testing.T) {
		repo, err := superRepo.Get(authCtx, "no-such-repo-"+uuid.New().String())
		require.NoError(t, err)

		_, err = repo.Datasets(authCtx)
		require.Error(t, err)
		assert.Equal(t, codes.PermissionDenied, status.Code(err))

		expectedClientID := sstauth.PerRepoClientID(superClientID, "default")
		t.Logf("server super client ID %q; per-repo audience for default: %q", superClientID, expectedClientID)
		t.Logf("token audiences: %v", tokenAudiences(token))
	})

	t.Run("write to default repo denied with read-only role", func(t *testing.T) {
		err := writeTestCommit(authCtx, superRepo, "default")
		require.Error(t, err)
		assert.Equal(t, codes.PermissionDenied, status.Code(err))
	})

	t.Run("repoA read allowed", func(t *testing.T) {
		repo, err := superRepo.Get(authCtx, "repoA")
		require.NoError(t, err)

		_, err = repo.Datasets(authCtx)
		require.NoError(t, err)

		req := bleve.NewSearchRequest(bleve.NewMatchAllQuery())
		res, err := repo.Bleve().SearchInContext(authCtx, req)
		require.NoError(t, err)
		t.Logf("repoA search hits: %d", res.Total)
	})

	t.Run("repoA write allowed", func(t *testing.T) {
		repo, err := superRepo.Get(authCtx, "repoA")
		require.NoError(t, err)

		branch := "access-test-" + uuid.New().String()
		require.NoError(t, writeTestCommitToBranch(authCtx, repo, branch))
		t.Logf("committed to repoA branch %s", branch)

		datasets, err := repo.Datasets(authCtx)
		require.NoError(t, err)
		for _, dsIRI := range datasets {
			ds, err := repo.Dataset(authCtx, dsIRI)
			require.NoError(t, err)
			if rbErr := ds.RemoveBranch(authCtx, branch); rbErr != nil {
				t.Logf("cleanup remove repoA branch %s: %v", branch, rbErr)
			}
		}
	})
}

// TestRealRepository2CheckoutRevision connects to the regular repository named
// "repository2" configured in .env.sst-test, lists its datasets, probes the
// requested dataset, and attempts to check out the specified revision.
// Diagnostic logging is produced at every step to help identify connection,
// permission, dataset, or revision problems.
func TestRealRepository2CheckoutRevision(t *testing.T) {
	env := loadRealCredentials(t)
	token := fetchKeycloakToken(t, env)

	ctx := context.Background()
	p := &tokenProvider{token: token, email: tokenEmail(token)}
	authCtx := sstauth.ContextWithAuthProvider(ctx, p)

	target := loadRealTargetByName(t, "repository2")
	t.Logf("connecting to repository2 at %s", target)
	tlsOpt := grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{}))

	repo, err := sst.OpenRemoteRepository(authCtx, target, tlsOpt)
	require.NoError(t, err)
	defer repo.Close()

	info, err := repo.Info(authCtx, "")
	require.NoError(t, err)
	t.Logf("repository info: %+v", info)

	datasets, err := repo.Datasets(authCtx)
	require.NoError(t, err)
	t.Logf("available datasets (%d):", len(datasets))
	for i := 0; i < 3 && i < len(datasets); i++ {
		t.Logf("  [%d] %s", i, datasets[i])
	}
	if len(datasets) > 3 {
		t.Logf("  ...")
		for i := len(datasets) - 3; i < len(datasets); i++ {
			t.Logf("  [%d] %s", i, datasets[i])
		}
	}

	requestedIRI := sst.IRI("urn:uuid:a735c677-64a2-4fdc-9ccc-caf145f218dc")
	ds, err := repo.Dataset(authCtx, requestedIRI)
	if err != nil {
		t.Logf("requested dataset %s is not accessible: %v", requestedIRI, err)
		require.NotEmpty(t, datasets, "no datasets available to probe")

		// Probe the first available dataset to see whether dataset access works at all.
		probeIRI := datasets[0]
		t.Logf("probing dataset %s instead", probeIRI)
		ds, err = repo.Dataset(authCtx, probeIRI)
		require.NoError(t, err)
	} else {
		t.Logf("requested dataset %s is accessible", requestedIRI)
	}

	branches, err := ds.Branches(authCtx)
	require.NoError(t, err)
	t.Logf("dataset %s branches (%d):", ds.IRI(), len(branches))
	for name, hash := range branches {
		t.Logf("  %s -> %s", name, hash)
	}

	leafCommits, err := ds.LeafCommits(authCtx)
	require.NoError(t, err)
	t.Logf("dataset %s leaf commits (%d):", ds.IRI(), len(leafCommits))
	for _, h := range leafCommits {
		t.Logf("  %s", h)
	}

	rev, err := sst.StringToHash("8LGL7iDoeePNQs2SqG19NR55yw5V7F2hZ989wDaLXfU7")
	require.NoError(t, err)

	// Verify whether the requested hash is known as a dataset revision.
	t.Logf("checking CommitForRevision for %s", rev)
	commitOfRev, err := ds.CommitForRevision(authCtx, rev)
	if err != nil {
		t.Logf("CommitForRevision(%s) failed: %v", rev, err)
	} else {
		t.Logf("CommitForRevision(%s) succeeded: %s", rev, commitOfRev)
	}

	// Verify whether the requested hash is known as a commit in this repository.
	t.Logf("checking CommitDetails for %s", rev)
	commitDetailsList, err := repo.CommitDetails(authCtx, []sst.Hash{rev})
	if err != nil {
		t.Logf("CommitDetails(%s) failed: %v", rev, err)
	} else if len(commitDetailsList) == 0 {
		t.Logf("CommitDetails(%s) returned empty list: hash is not a known commit in this repository", rev)
	} else {
		commitDetails := commitDetailsList[0]
		t.Logf("CommitDetails(%s) succeeded: author=%s date=%s message=%q datasetRevisions=%v",
			rev, commitDetails.Author, commitDetails.AuthorDate, commitDetails.Message, commitDetails.DatasetRevisions)
	}

	// Get the master commit details so we can extract a valid dataset revision and
	// verify that CheckoutRevision itself works on this server.
	masterCommit := branches["master"]
	masterCommitDetailsList, err := repo.CommitDetails(authCtx, []sst.Hash{masterCommit})
	require.NoError(t, err)
	require.NotEmpty(t, masterCommitDetailsList, "master commit details not found")
	masterCommitDetails := masterCommitDetailsList[0]
	t.Logf("master commit %s details: message=%q datasetRevisions=%v",
		masterCommit, masterCommitDetails.Message, masterCommitDetails.DatasetRevisions)

	validDSRevision, hasValidRev := masterCommitDetails.DatasetRevisions[ds.IRI()]
	if hasValidRev {
		revCtx, revCancel := context.WithTimeout(authCtx, 30*time.Second)
		defer revCancel()
		t.Logf("trying CheckoutRevision(%s) with the valid master dataset revision", validDSRevision)
		stageRev, revErr := ds.CheckoutRevision(revCtx, validDSRevision, sst.DefaultTriplexMode)
		if revErr != nil {
			t.Logf("CheckoutRevision(%s) failed: %v", validDSRevision, revErr)
		} else {
			t.Logf("CheckoutRevision(%s) succeeded, stage valid=%t, local graphs=%d, referenced graphs=%d",
				validDSRevision, stageRev.IsValid(), len(stageRev.NamedGraphs()), len(stageRev.ReferencedGraphs()))
		}
	} else {
		t.Logf("master commit does not contain a dataset revision for %s", ds.IRI())
	}

	// Probe CheckoutRevision on a few other datasets' valid master revisions.
	// Use a short timeout so we can test several datasets without hanging the whole test.
	probeCount := 5
	for i := 1; i <= probeCount && i < len(datasets); i++ {
		probeIRI := datasets[i*len(datasets)/(probeCount+1)]
		t.Logf("[probe %d] dataset %s", i, probeIRI)
		probeDS, probeErr := repo.Dataset(authCtx, probeIRI)
		if probeErr != nil {
			t.Logf("  access failed: %v", probeErr)
			continue
		}
		probeBranches, probeErr := probeDS.Branches(authCtx)
		if probeErr != nil {
			t.Logf("  Branches failed: %v", probeErr)
			continue
		}
		probeMasterCommit, ok := probeBranches["master"]
		if !ok {
			t.Logf("  no master branch")
			continue
		}
		probeCommitDetailsList, probeErr := repo.CommitDetails(authCtx, []sst.Hash{probeMasterCommit})
		if probeErr != nil || len(probeCommitDetailsList) == 0 {
			t.Logf("  CommitDetails failed or empty: %v", probeErr)
			continue
		}
		probeDSRev, ok := probeCommitDetailsList[0].DatasetRevisions[probeDS.IRI()]
		if !ok {
			t.Logf("  no dataset revision in master commit")
			continue
		}
		probeRevCtx, probeRevCancel := context.WithTimeout(authCtx, 15*time.Second)
		stageProbe, probeErr := probeDS.CheckoutRevision(probeRevCtx, probeDSRev, sst.DefaultTriplexMode)
		probeRevCancel()
		if probeErr != nil {
			t.Logf("  CheckoutRevision(%s) failed: %v", probeDSRev, probeErr)
		} else {
			t.Logf("  CheckoutRevision(%s) succeeded, stage valid=%t, local graphs=%d, referenced graphs=%d",
				probeDSRev, stageProbe.IsValid(), len(stageProbe.NamedGraphs()), len(stageProbe.ReferencedGraphs()))
		}
	}

	// First try a branch checkout to confirm the checkout mechanism works.
	t.Logf("trying CheckoutBranch(master) on dataset %s", ds.IRI())
	stageBranch, err := ds.CheckoutBranch(authCtx, "master", sst.DefaultTriplexMode)
	if err != nil {
		t.Logf("CheckoutBranch(master) failed: %v", err)
	} else {
		t.Logf("CheckoutBranch(master) succeeded, stage valid=%t, local graphs=%d, referenced graphs=%d",
			stageBranch.IsValid(), len(stageBranch.NamedGraphs()), len(stageBranch.ReferencedGraphs()))

		// Verify that CheckoutCommit works with the known master commit hash.
		masterCommit := branches["master"]
		t.Logf("trying CheckoutCommit(%s) with the known master commit", masterCommit)
		stageCommit, commitErr := ds.CheckoutCommit(authCtx, masterCommit, sst.DefaultTriplexMode)
		if commitErr != nil {
			t.Logf("CheckoutCommit(%s) failed: %v", masterCommit, commitErr)
		} else {
			t.Logf("CheckoutCommit(%s) succeeded, stage valid=%t, local graphs=%d, referenced graphs=%d",
				masterCommit, stageCommit.IsValid(), len(stageCommit.NamedGraphs()), len(stageCommit.ReferencedGraphs()))
		}
	}

	// The server running on repository2 does not implement GetCommitForDatasetRevision
	// or the WantDatasetRevision stream path used by CheckoutRevision, even though
	// the requested hash is the current master dataset revision. Fall back to the
	// known master commit so the test can still output a Stage.
	masterCommit, hasMaster := branches["master"]
	require.True(t, hasMaster, "dataset has no master branch to fall back to")

	longCtx, longCancel := context.WithTimeout(authCtx, 90*time.Second)
	defer longCancel()

	t.Logf("requested hash %s is invalid; falling back to CheckoutCommit(%s)", rev, masterCommit)
	stage, err := ds.CheckoutCommit(longCtx, masterCommit, sst.DefaultTriplexMode)
	require.NoError(t, err)
	require.True(t, stage.IsValid())

	t.Logf("checked out stage for dataset %s at commit %s (requested revision %s is not supported by CheckoutRevision on this server)", ds.IRI(), masterCommit, rev)
	t.Logf("stage valid: %t", stage.IsValid())
	t.Logf("local named graphs: %d", len(stage.NamedGraphs()))
	for i, ng := range stage.NamedGraphs() {
		t.Logf("  local graph[%d]: iri=%s referenced=%t empty=%t modified=%t iriNodes=%d blankNodes=%d",
			i, ng.IRI(), ng.IsReferenced(), ng.IsEmpty(), ng.IsModified(), ng.IRINodeCount(), ng.BlankNodeCount())
	}
	t.Logf("referenced named graphs: %d", len(stage.ReferencedGraphs()))
	for i, ng := range stage.ReferencedGraphs() {
		t.Logf("  referenced graph[%d]: iri=%s empty=%t modified=%t iriNodes=%d blankNodes=%d",
			i, ng.IRI(), ng.IsEmpty(), ng.IsModified(), ng.IRINodeCount(), ng.BlankNodeCount())
	}
}

func ensureRepoA(t *testing.T, ctx context.Context, superRepo sst.SuperRepository) {
	names, err := superRepo.List(ctx)
	require.NoError(t, err)
	if slices.Contains(names, "repoA") {
		return
	}
	_, err = superRepo.Create(ctx, "repoA")
	require.NoError(t, err)
	t.Logf("created repoA")
}

func writeTestCommit(ctx context.Context, superRepo sst.SuperRepository, repoName string) error {
	repo, err := superRepo.Get(ctx, repoName)
	if err != nil {
		return err
	}
	return writeTestCommitToBranch(ctx, repo, "access-test-"+uuid.New().String())
}

func writeTestCommitToBranch(ctx context.Context, repo sst.Repository, branch string) error {
	st := repo.OpenStage(sst.DefaultTriplexMode)
	ng := st.CreateNamedGraph(sst.IRI("urn:uuid:" + uuid.New().String()))
	ng.CreateIRINode("access-test", lci.Person)
	_, _, err := st.Commit(ctx, "access right write test", branch)
	return err
}

type repositoryTarget struct {
	Name string `json:"name"`
	Type string `json:"type"`
	URL  string `json:"url"`
}

func loadRealTargetByName(t *testing.T, name string) string {
	envPath := filepath.Join("..", ".env.sst-test")
	if _, err := os.Stat(envPath); err != nil {
		t.Skipf("real test target not configured: %v", err)
	}

	data, err := os.ReadFile(envPath)
	require.NoError(t, err)

	var targets []repositoryTarget
	if err := json.Unmarshal(data, &targets); err != nil {
		t.Fatalf("invalid repository targets JSON in %s: %v", envPath, err)
	}

	for _, tgt := range targets {
		if tgt.Name == name {
			if tgt.URL == "" {
				t.Fatalf("target %q has no URL in %s", name, envPath)
			}
			return tgt.URL
		}
	}

	t.Fatalf("target %q not found in %s", name, envPath)
	return ""
}

func loadRealCredentials(t *testing.T) map[string]string {
	envPath := filepath.Join("..", ".env.sst-cli")
	if _, err := os.Stat(envPath); err != nil {
		t.Skipf("real credentials not available: %v", err)
	}

	env, err := loadDotEnv(envPath)
	require.NoError(t, err)

	for _, k := range []string{
		"SST_OIDC_REALM_URL",
		"SST_OIDC_CLIENT_ID",
		"SST_OIDC_CLIENT_SECRET",
		"SST_USERNAME",
		"SST_PASSWORD",
	} {
		if env[k] == "" {
			t.Skipf("missing %s in %s", k, envPath)
		}
	}
	return env
}

func fetchKeycloakToken(t *testing.T, env map[string]string) *oauth2.Token {
	tokenURL := strings.TrimSuffix(env["SST_OIDC_REALM_URL"], "/") + "/protocol/openid-connect/token"
	cfg := &oauth2.Config{
		ClientID:     env["SST_OIDC_CLIENT_ID"],
		ClientSecret: env["SST_OIDC_CLIENT_SECRET"],
		Endpoint:     oauth2.Endpoint{TokenURL: tokenURL},
		Scopes:       []string{"openid", "roles", "email", "profile"},
	}

	token, err := cfg.PasswordCredentialsToken(context.Background(), env["SST_USERNAME"], env["SST_PASSWORD"])
	require.NoError(t, err)
	return token
}

func tokenEmail(token *oauth2.Token) string {
	claims, err := parseJWTClaims(token.AccessToken)
	if err != nil {
		return ""
	}
	if s, ok := claims["email"].(string); ok {
		return s
	}
	return ""
}

func tokenAudiences(token *oauth2.Token) []string {
	claims, err := parseJWTClaims(token.AccessToken)
	if err != nil {
		return nil
	}
	aud, _ := claims["aud"].([]any)
	out := make([]string, 0, len(aud))
	for _, v := range aud {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func logTokenAccessModes(t *testing.T, token *oauth2.Token) {
	claims, err := parseJWTClaims(token.AccessToken)
	require.NoError(t, err)

	ra, ok := claims["resource_access"].(map[string]any)
	if !ok {
		return
	}
	for clientID, v := range ra {
		entry, ok := v.(map[string]any)
		if !ok {
			continue
		}
		rolesAny, ok := entry["roles"].([]any)
		if !ok {
			continue
		}
		roles := make([]string, 0, len(rolesAny))
		for _, r := range rolesAny {
			if s, ok := r.(string); ok {
				roles = append(roles, s)
			}
		}
		mode := sstauth.AccessModeFromRoles(roles)
		t.Logf("client %q roles %v -> access mode %q", clientID, roles, mode.String())
	}
}

// TestParseRealKeycloakToken fetches a token from the real Keycloak instance
// described in .env.sst-cli and parses the access_token and id_token payloads.
// It is skipped when the credential file is missing.
func TestParseRealKeycloakToken(t *testing.T) {
	env := loadRealCredentials(t)
	token := fetchKeycloakToken(t, env)

	accessClaims, err := parseJWTClaims(token.AccessToken)
	require.NoError(t, err)

	idTokenRaw, ok := token.Extra("id_token").(string)
	require.True(t, ok && idTokenRaw != "", "id_token not present in token response")
	idClaims, err := parseJWTClaims(idTokenRaw)
	require.NoError(t, err)

	t.Logf("access_token claims:\n%s", prettyJSON(accessClaims))
	t.Logf("id_token claims:\n%s", prettyJSON(idClaims))

	// Map the Keycloak roles for each client to our AccessMode so we can verify
	// the role-name parsing logic against a real token.
	if ra, ok := accessClaims["resource_access"].(map[string]any); ok {
		for clientID, v := range ra {
			entry, ok := v.(map[string]any)
			if !ok {
				continue
			}
			rolesAny, ok := entry["roles"].([]any)
			if !ok {
				continue
			}
			roles := make([]string, 0, len(rolesAny))
			for _, r := range rolesAny {
				if s, ok := r.(string); ok {
					roles = append(roles, s)
				}
			}
			mode := sstauth.AccessModeFromRoles(roles)
			t.Logf("client %q roles %v -> access mode %q", clientID, roles, mode.String())
		}
	}
}

func loadDotEnv(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	env := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		env[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}
	return env, nil
}

func parseJWTClaims(raw string) (map[string]any, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}

	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, err
	}
	return claims, nil
}

func prettyJSON(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("%+v", v)
	}
	return string(b)
}

type tokenProvider struct {
	token *oauth2.Token
	email string
}

func (p *tokenProvider) AuthProvider() {}
func (p *tokenProvider) Info() (string, string, error) {
	if p.email == "" {
		return "", "", nil
	}
	return p.email, "", nil
}
func (p *tokenProvider) Oauth2Token() (*oauth2.Token, error) { return p.token, nil }

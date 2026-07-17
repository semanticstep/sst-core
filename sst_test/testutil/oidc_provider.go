// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package testutil

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/semanticstep/sst-core/defaultderive"
	"github.com/semanticstep/sst-core/sst"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// FakeOIDCProvider is a minimal OIDC provider for tests. It exposes the
// discovery document and a JWKS endpoint required by go-oidc and can issue
// signed JWTs that carry Keycloak-style resource_access claims.
type FakeOIDCProvider struct {
	issuer string
	server *http.Server
	key    *rsa.PrivateKey
	kid    string
	mu     sync.Mutex
}

// StartFakeOIDCProvider starts an HTTP server that mimics the endpoints used
// by go-oidc. The returned provider must be closed by the test cleanup.
func StartFakeOIDCProvider(t testing.TB) *FakeOIDCProvider {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	p := &FakeOIDCProvider{
		key: key,
		kid: "test-key",
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", p.discoveryHandler)
	mux.HandleFunc("/keys", p.jwksHandler)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	p.issuer = "http://" + lis.Addr().String()
	p.server = &http.Server{Handler: mux}

	go func() {
		if err := p.server.Serve(lis); err != nil && err != http.ErrServerClosed {
			t.Logf("fake OIDC provider serve error: %v", err)
		}
	}()

	t.Cleanup(func() {
		if p.server != nil {
			_ = p.server.Close()
		}
	})

	return p
}

// Issuer returns the issuer URL used in discovery documents and tokens.
func (p *FakeOIDCProvider) Issuer() string {
	return p.issuer
}

// IssueToken returns a raw signed JWT with the requested audience and
// Keycloak-style resource_access claim.
func (p *FakeOIDCProvider) IssueToken(email string, audience []string, resourceAccess map[string][]string) string {
	now := time.Now()

	header := map[string]string{
		"alg": "RS256",
		"typ": "JWT",
		"kid": p.kid,
	}

	raClaims := make(map[string]struct {
		Roles []string `json:"roles"`
	}, len(resourceAccess))
	for clientID, roles := range resourceAccess {
		raClaims[clientID] = struct {
			Roles []string `json:"roles"`
		}{Roles: roles}
	}

	claims := map[string]any{
		"iss":             p.issuer,
		"sub":             "test-user",
		"aud":             audience,
		"exp":             now.Add(time.Hour).Unix(),
		"iat":             now.Unix(),
		"email":           email,
		"resource_access": raClaims,
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		panic(err)
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		panic(err)
	}

	encodedHeader := base64.RawURLEncoding.EncodeToString(headerJSON)
	encodedClaims := base64.RawURLEncoding.EncodeToString(claimsJSON)
	signingInput := encodedHeader + "." + encodedClaims

	hash := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, p.key, crypto.SHA256, hash[:])
	if err != nil {
		panic(err)
	}

	encodedSignature := base64.RawURLEncoding.EncodeToString(signature)
	return signingInput + "." + encodedSignature
}

func (p *FakeOIDCProvider) discoveryHandler(w http.ResponseWriter, _ *http.Request) {
	p.mu.Lock()
	defer p.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"issuer":                 p.issuer,
		"authorization_endpoint": p.issuer + "/auth",
		"token_endpoint":         p.issuer + "/token",
		"userinfo_endpoint":      p.issuer + "/userinfo",
		"jwks_uri":               p.issuer + "/keys",
	})
}

func (p *FakeOIDCProvider) jwksHandler(w http.ResponseWriter, _ *http.Request) {
	p.mu.Lock()
	defer p.mu.Unlock()

	n := base64.RawURLEncoding.EncodeToString(p.key.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(p.key.E)).Bytes())

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"keys": []map[string]any{
			{
				"kty": "RSA",
				"alg": "RS256",
				"use": "sig",
				"kid": p.kid,
				"n":   n,
				"e":   e,
			},
		},
	})
}

// SuperServerServeWithPerRepoAuth starts a SuperRepositoryServer with OIDC
// enabled and per-repository authorization active. It returns the gRPC server
// address and the fake OIDC provider used to sign test tokens.
func SuperServerServeWithPerRepoAuth(t testing.TB, path string, clientID string) (string, *FakeOIDCProvider) {
	cert, err := TestServerCert()
	require.NoError(t, err)

	provider := StartFakeOIDCProvider(t)

	server, err := sst.NewSuperServer(&sst.RepositoryServerConfig{
		RepoDir:    path,
		Issuer:     provider.Issuer(),
		ClientID:   clientID,
		ServerCert: &cert,
		Verbose:    true,
		DeriveInfo: defaultderive.DeriveInfo(),
	})
	require.NoError(t, err)

	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)

	port := lis.Addr().(*net.TCPAddr).Port
	url := fmt.Sprintf("localhost:%d", port)

	go func() {
		assert.NoError(t, server.Serve(lis))
	}()
	t.Cleanup(func() {
		require.NoError(t, server.GracefulStopAndClose())
	})

	return url, provider
}

// ServerServeWithOIDC starts a single RepositoryServer with OIDC enabled and
// per-repository authorization disabled. It returns the gRPC server address
// and the fake OIDC provider used to sign test tokens.
func ServerServeWithOIDC(t testing.TB, path string, clientID string) (string, *FakeOIDCProvider) {
	cert, err := TestServerCert()
	require.NoError(t, err)

	provider := StartFakeOIDCProvider(t)

	server, err := sst.NewServer(&sst.RepositoryServerConfig{
		RepoDir:    path,
		Issuer:     provider.Issuer(),
		ClientID:   clientID,
		ServerCert: &cert,
		Verbose:    true,
		DeriveInfo: defaultderive.DeriveInfo(),
	})
	require.NoError(t, err)

	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)

	port := lis.Addr().(*net.TCPAddr).Port
	url := fmt.Sprintf("localhost:%d", port)

	go func() {
		assert.NoError(t, server.Serve(lis))
	}()
	t.Cleanup(func() {
		require.NoError(t, server.GracefulStopAndClose())
	})

	return url, provider
}

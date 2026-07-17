// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sstauth

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	cache "github.com/go-pkgz/expirable-cache/v2"
	grpc_auth "github.com/grpc-ecosystem/go-grpc-middleware/auth"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	tokenVerifierUpdatePeriod = 30 * time.Second
	maxTokenCacheSize         = 128
	tokenExpiryDelta          = 10 * time.Second

	// perRepoClientIDSeparator is the delimiter between the super client ID and
	// the repository name when building per-repository Keycloak client IDs.
	perRepoClientIDSeparator = "#"
)

// PerRepoClientID returns the Keycloak client ID / token audience used for a
// single repository managed by a SuperRepository.
func PerRepoClientID(superClientID, repoName string) string {
	return superClientID + perRepoClientIDSeparator + repoName
}

// IsPerRepoClientID reports whether audience is a per-repository client ID for
// the given super client ID.
func IsPerRepoClientID(superClientID, audience string) bool {
	return strings.HasPrefix(audience, superClientID+perRepoClientIDSeparator)
}

type Provider interface {
	Oauth2Token() (*oauth2.Token, error)
}

type tokenToUserInfoFuncForTest func(ctx context.Context, rawToken, issuer string) (SstUserInfo, error)

type tokenVerifierHolder struct {
	value        atomic.Value
	updateTicker *time.Ticker
}

// AuthFunc returns a grpc_auth.AuthFunc that authenticates a token from the
// context metadata and converts it to SstUserInfo.
//
// Parameters:
// - issuer: A string representing the token issuer.
// - tokenToUserInfo: A function that converts a token to SstUserInfo.
//
// The returned grpc_auth.AuthFunc extracts the token from the context metadata,
// verifies it, and converts it to authentication information. If the token is
// invalid, it returns an error with the Unauthenticated status code.
//
// The function uses a tokenVerifierHolder to manage token verification and
// periodically updates the token verifier.
func AuthFunc(issuer string, testOverride tokenToUserInfoFuncForTest) grpc_auth.AuthFunc {
	tokenVerifierHolder := tokenVerifierHolder{
		updateTicker: time.NewTicker(tokenVerifierUpdatePeriod),
	}
	return func(ctx context.Context) (context.Context, error) {
		// AuthFromMD is a helper function for extracting the :authorization header from the gRPC metadata of the request.
		// It expects the `:authorization` header to be of a certain scheme (e.g. `basic`, `bearer`),
		// in a case-insensitive format (see rfc2617, sec 1.2). If no such authorization is found,
		// or the token is of wrong scheme, an error with gRPC status `Unauthenticated` is returned.
		rawToken, err := grpc_auth.AuthFromMD(ctx, "bearer")
		if err != nil {
			return nil, err
		}

		userInfo, err := verifyTokenAndClaimSstUserInfo(ctx, rawToken, issuer, &tokenVerifierHolder, testOverride)
		if err != nil {
			return nil, status.Errorf(codes.Unauthenticated, "invalid auth token: %v", err)
		}

		return ContextWithSstUserInfo(ctx, &SstUserInfo{Email: userInfo.Email}), nil
	}
}

type provider struct {
	// rawToken: access token
	rawToken string

	// info: returns Email and Name of the user
	info func() (email string, name string, err error)
}

func (p provider) AuthProvider()                                {}
func (p provider) Info() (email string, name string, err error) { return p.info() }
func (p provider) Oauth2Token() (*oauth2.Token, error) {
	return &oauth2.Token{AccessToken: p.rawToken}, nil
}

// ProviderFromHeader creates a middleware that extracts an access token
// from the request header. Then, it adds the authentication provider(includes access token)
// to the request context and calls the next handler.
//
// Parameters:
//   - issuer: The expected issuer of the token; e.g. the KeyCloak URL to use
//
// Returns:
//
// The returned middleware function modify the handler for the http.Request and the http.Response.
func ProviderFromHeader(issuer string) func(http.Handler) http.Handler {
	tokenVerifierHolder := tokenVerifierHolder{
		updateTicker: time.NewTicker(tokenVerifierUpdatePeriod),
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rawToken := authTokenFromHeader(r)
			if rawToken == "" {
				http.Error(w, "401 unauthorized", http.StatusUnauthorized)
				return
			}
			ctx := r.Context()
			next.ServeHTTP(w, r.WithContext(ContextWithAuthProvider(ctx, provider{
				rawToken: rawToken,
				info: func() (email string, name string, err error) {
					info, err := verifyTokenAndClaimSstUserInfo(ctx, rawToken, issuer, &tokenVerifierHolder, nil)
					email = info.Email
					return
				},
			})))
		})
	}
}

// get Authorization access token from request if provided and return it, otherwise return an empty string.
func authTokenFromHeader(r *http.Request) string {
	bearer := r.Header.Get("Authorization")
	if len(bearer) > 7 && strings.ToLower(bearer[0:7]) == "bearer " {
		return bearer[7:]
	}
	return ""
}

// call oidcTokenVerificationAndCaching to verify the rawToken and get SstUserInfo.
func verifyTokenAndClaimSstUserInfo(
	ctx context.Context,
	rawToken, issuer string,
	tokenVerifierHolder *tokenVerifierHolder,
	testOverride tokenToUserInfoFuncForTest,
) (*SstUserInfo, error) {
	var authInfo SstUserInfo
	var err error
	if testOverride != nil {
		authInfo, err = testOverride(ctx, rawToken, issuer)
		if err != nil {
			return &authInfo, err
		}
	} else {
		var t *tokenVerifier
		select {
		case <-tokenVerifierHolder.updateTicker.C:
			t = &tokenVerifier{
				tokenCache: cache.NewCache[string, *oidc.IDToken]().WithMaxKeys(maxTokenCacheSize).WithLRU(),
			}
			t.init(ctx, issuer)
			if t.err == nil || !errors.Is(t.err, context.Canceled) {
				tokenVerifierHolder.value.Store(t)
			}
		default:
			t, _ = tokenVerifierHolder.value.Load().(*tokenVerifier)
			if t == nil {
				t = &tokenVerifier{
					tokenCache: cache.NewCache[string, *oidc.IDToken]().WithMaxKeys(maxTokenCacheSize).WithLRU(),
				}
				t.init(ctx, issuer)
				if t.err == nil || !errors.Is(t.err, context.Canceled) {
					tokenVerifierHolder.value.Store(t)
					tokenVerifierHolder.updateTicker.Reset(tokenVerifierUpdatePeriod)
				}
			}
		}
		if t.err != nil {
			return nil, t.err
		}
		authInfo, err = oidcTokenVerificationAndCaching(ctx, rawToken, t.verifier, t.tokenCache)
		if err != nil {
			return nil, err
		}
	}
	return &authInfo, nil
}

type tokenVerifier struct {
	err        error
	verifier   *oidc.IDTokenVerifier
	tokenCache cache.Cache[string, *oidc.IDToken]
}

func NewOIDCVerifier(ctx context.Context, issuer string, clientID string) (*oidc.IDTokenVerifier, error) {
	if issuer == "" {
		return nil, errors.New("issuer required")
	}
	if clientID == "" {
		return nil, errors.New("clientID required")
	}
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, err
	}
	// Skip the default audience check because per-repo tokens have the repo URL
	// as their audience, not the fixed super client ID. The RBAC interceptors
	// validate the audience manually against the expected client ID for each
	// request.
	return provider.Verifier(&oidc.Config{SkipClientIDCheck: true}), nil
}

func (t *tokenVerifier) init(ctx context.Context, issuer string) {
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		t.err = err
		return
	}
	t.verifier = provider.Verifier(&oidc.Config{
		SkipClientIDCheck: true,
	})
}

type KCClaims struct {
	Email string `json:"email"`
	Name  string `json:"name"`

	// resource_access.<clientId>.roles
	ResourceAccess map[string]struct {
		Roles []string `json:"roles"`
	} `json:"resource_access"`
}

func RolesForClient(c KCClaims, clientID string) map[string]bool {
	out := map[string]bool{}
	if ra, ok := c.ResourceAccess[clientID]; ok {
		for _, r := range ra.Roles {
			out[r] = true
		}
	}
	return out
}

// oidcTokenVerificationAndCaching verifies an OIDC token and caches the result.
// If the token is found in the cache, it returns the cached token information.
// Otherwise, it verifies the token using the provided token verifier, caches the result, and returns the token information.
//
// Parameters:
//   - ctx: The context for the request.
//   - rawToken: The raw OIDC token string to be verified.
//   - tokenVerifier: The OIDC token verifier to use for token verification.
//   - tokenCache: The cache to store and retrieve verified tokens.
//
// Returns:
//   - info: The user information extracted from the token claims.
//   - error: An error if the token verification or claims extraction fails.
func oidcTokenVerificationAndCaching(
	ctx context.Context, rawToken string, tokenVerifier *oidc.IDTokenVerifier, tokenCache cache.Cache[string, *oidc.IDToken],
) (info SstUserInfo, err error) {
	idToken, found := tokenCache.Get(rawToken)
	if !found {
		it, err := tokenVerifier.Verify(ctx, rawToken)
		if err != nil {
			return info, err
		}
		idToken = it
		// the cache expiration must not be bigger than the token expiration. Why not use the same value?
		// tokenExpiryDelta is 10 * time.Second now.
		tokenCache.Set(rawToken, idToken, time.Until(idToken.Expiry.Round(0).Add(-tokenExpiryDelta)))
	}
	var c KCClaims
	if err := idToken.Claims(&c); err != nil {
		return info, err
	}

	return SstUserInfo{Email: c.Email}, nil
}

func tokenFromIncomingContext(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "missing metadata")
	}

	// gRPC metadata keys are lowercase
	vals := md.Get("authorization")
	if len(vals) == 0 {
		return "", status.Error(codes.Unauthenticated, "missing authorization")
	}

	// Expect: "Bearer <token>"
	v := strings.TrimSpace(vals[0])
	parts := strings.SplitN(v, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", status.Error(codes.Unauthenticated, "invalid authorization format")
	}
	if parts[1] == "" {
		return "", status.Error(codes.Unauthenticated, "empty token")
	}
	return parts[1], nil
}

// RepoNameFromRequest extracts the RepoName field from any gRPC request that
// exposes GetRepoName(). If the request has no RepoName or it is empty, it
// returns "default".
func RepoNameFromRequest(req any) string {
	type repoNamer interface{ GetRepoName() string }
	if r, ok := req.(repoNamer); ok {
		if name := r.GetRepoName(); name != "" {
			return name
		}
	}
	return "default"
}

// AccessModeForClientID returns the effective access mode for the given
// Keycloak client ID by looking up roles in the token claims.
func AccessModeForClientID(claims KCClaims, clientID string) AccessMode {
	roles := RolesForClient(claims, clientID)
	roleList := make([]string, 0, len(roles))
	for r := range roles {
		roleList = append(roleList, r)
	}
	return AccessModeFromRoles(roleList)
}

// CheckRepoAccess validates that the principal in ctx has the required access
// mode for the requested repo URL. It is intended for streaming RPC handlers
// that receive the repo name in their first message.
//
// If useSuperClient is true, repoURL is ignored and the super client ID is used.
// Otherwise repoURL is expected to be the full repo client ID (superClientID + "#" + repoName).
func CheckRepoAccess(
	ctx context.Context,
	repoURL string,
	fullMethod string,
	methodRoles map[string]AccessMode,
	superClientID string,
	useSuperClient bool,
) error {
	p, ok := PrincipalFromContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "missing principal")
	}

	expectedClientID := superClientID
	if !useSuperClient {
		expectedClientID = repoURL
	}

	if !slices.Contains(p.Audience, expectedClientID) {
		return status.Error(codes.PermissionDenied, "invalid audience")
	}

	mode := AccessModeForClientID(p.Claims, expectedClientID)
	if required, found := methodRoles[fullMethod]; found {
		if !HasAccess(mode, required) {
			return status.Error(codes.PermissionDenied, "forbidden")
		}
	}
	return nil
}

func UnaryRBACInterceptor(
	verifier *oidc.IDTokenVerifier,
	superClientID string,
	perRepoAuth bool,
	methodRoles map[string]AccessMode,
) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		tok, err := tokenFromIncomingContext(ctx)
		if err != nil {
			return nil, err
		}

		idToken, err := verifier.Verify(ctx, tok)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, err.Error())
		}

		var c KCClaims
		if err := idToken.Claims(&c); err != nil {
			return nil, status.Error(codes.Unauthenticated, "bad claims")
		}

		// SuperRepository management methods always use the super client ID.
		// Otherwise, when per-repo auth is enabled, resolve the target repo's
		// client ID from the request.
		useSuperClient := !perRepoAuth || strings.HasPrefix(info.FullMethod, "/sst.repository.RepoManagerService/")
		expectedClientID := superClientID
		if !useSuperClient {
			repoName := RepoNameFromRequest(req)
			expectedClientID = PerRepoClientID(superClientID, repoName)
		}

		if !slices.Contains(idToken.Audience, expectedClientID) {
			return nil, status.Error(codes.PermissionDenied, "invalid audience")
		}

		mode := AccessModeForClientID(c, expectedClientID)

		// Method-level RBAC
		if required, ok := methodRoles[info.FullMethod]; ok {
			if !HasAccess(mode, required) {
				return nil, status.Error(codes.PermissionDenied, "forbidden")
			}
		}

		p := &Principal{Email: c.Email, Name: c.Name, Claims: c, Audience: idToken.Audience}
		return handler(WithPrincipal(ctx, p), req)
	}
}

type Principal struct {
	Email    string
	Name     string
	Claims   KCClaims
	Audience []string
}

type ctxKey int

const principalKey ctxKey = 1

func WithPrincipal(ctx context.Context, p *Principal) context.Context {
	return context.WithValue(ctx, principalKey, p)
}

func PrincipalFromContext(ctx context.Context) (*Principal, bool) {
	p, ok := ctx.Value(principalKey).(*Principal)
	return p, ok
}

func StreamRBACInterceptor(
	verifier *oidc.IDTokenVerifier,
	superClientID string,
	perRepoAuth bool,
	methodRoles map[string]AccessMode,
) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		tok, err := tokenFromIncomingContext(ss.Context())
		if err != nil {
			return err
		}

		idToken, err := verifier.Verify(ss.Context(), tok)
		if err != nil {
			return status.Error(codes.Unauthenticated, err.Error())
		}

		var c KCClaims
		if err := idToken.Claims(&c); err != nil {
			return status.Error(codes.Unauthenticated, "bad claims")
		}

		// For streaming RPCs the target repo name arrives in the first message,
		// so we can only verify that the token belongs to this server (super
		// client or any per-repo client) and that the principal carries the
		// audience list. The per-repo access check (audience + roles) is done
		// by the handler after reading the first message.
		if perRepoAuth {
			audienceOK := false
			for _, aud := range idToken.Audience {
				if aud == superClientID || IsPerRepoClientID(superClientID, aud) {
					audienceOK = true
					break
				}
			}
			if !audienceOK {
				return status.Error(codes.PermissionDenied, "invalid audience")
			}
		} else {
			if !slices.Contains(idToken.Audience, superClientID) {
				return status.Error(codes.PermissionDenied, "invalid audience")
			}
		}

		methodRequired, hasMethodRole := methodRoles[info.FullMethod]
		if hasMethodRole {
			// When per-repo auth is enabled and the token only carries a
			// per-repo audience, the target repo is unknown until the first
			// message. The handler will enforce the per-repo method role there.
			// Only enforce super-client roles here when the token actually
			// targets the super client.
			enforceSuperRole := !perRepoAuth || slices.Contains(idToken.Audience, superClientID)
			if enforceSuperRole {
				superMode := AccessModeForClientID(c, superClientID)
				if !HasAccess(superMode, methodRequired) {
					return status.Error(codes.PermissionDenied, "forbidden")
				}
			}
		}

		p := &Principal{Email: c.Email, Name: c.Name, Claims: c, Audience: idToken.Audience}
		wrapped := &wrappedStream{ServerStream: ss, ctx: WithPrincipal(ss.Context(), p)}

		return handler(srv, wrapped)
	}
}

type wrappedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedStream) Context() context.Context { return w.ctx }

type errorKey struct{}

func errorFromContext(ctx context.Context) error {
	err := ctx.Value(errorKey{})
	if err, ok := err.(error); ok {
		return err
	}
	return nil
}

type client struct {
	Issuer, ClientID, ClientSecret string
}

func sstRefreshTokenSource(ctx context.Context, c client, refreshToken string) (oauth2.TokenSource, error) {
	fail := func(err error) (oauth2.TokenSource, error) { return nil, err }
	op, err := oidc.NewProvider(ctx, c.Issuer)
	if err != nil {
		return fail(err)
	}
	cfg := oauth2.Config{
		ClientID:     c.ClientID,
		ClientSecret: c.ClientSecret,
		Endpoint:     op.Endpoint(),
	}
	return oauth2.ReuseTokenSource(nil, cfg.TokenSource(ctx, &oauth2.Token{RefreshToken: refreshToken})), nil
}

func clientCredentialsTokenSource(ctx context.Context, c client) (oauth2.TokenSource, error) {
	fail := func(err error) (oauth2.TokenSource, error) { return nil, err }
	op, err := oidc.NewProvider(ctx, c.Issuer)
	if err != nil {
		return fail(err)
	}
	cfg := clientcredentials.Config{
		ClientID:     c.ClientID,
		ClientSecret: c.ClientSecret,
		TokenURL:     op.Endpoint().TokenURL,
		Scopes:       []string{"openid"},
	}
	return oauth2.ReuseTokenSource(nil, cfg.TokenSource(ctx)), nil
}

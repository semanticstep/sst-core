// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package utils

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/semanticstep/sst-core/sst"
	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
	"golang.org/x/term"
)

// GetUserCredentials prompts the user to enter their username and password
func GetUserCredentials() (string, string) {
	//auto login SST_USERNAME=xxxx SST_PASSWORD=xxxx sst interactive
	if u := os.Getenv("SST_USERNAME"); u != "" {
		if p := os.Getenv("SST_PASSWORD"); p != "" {
			return u, p
		}
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter Username: ")
	username, _ := reader.ReadString('\n')
	username = strings.TrimSpace(username)

	fmt.Print("Enter Password: ")
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		fmt.Println("\nError reading password:", err)
		return "", ""
	}
	fmt.Println()

	password := strings.TrimSpace(string(bytePassword))
	// password, _ := reader.ReadString('\n')
	// password = strings.TrimSpace(password)

	return username, password
}

// keycloakOIDCTokenPath is the fixed OpenID Connect token endpoint path on Keycloak realms.
const keycloakOIDCTokenPath = "/protocol/openid-connect/token"

type oidcConfig struct {
	realmURL     string
	clientID     string
	clientSecret string
}

func loadOIDCConfig() (oidcConfig, error) {
	realmURL := strings.TrimRight(strings.TrimSpace(os.Getenv("SST_OIDC_REALM_URL")), "/")
	if realmURL == "" {
		return oidcConfig{}, fmt.Errorf("SST_OIDC_REALM_URL is not set; set it in .env.sst-cli (repo root or $HOME)")
	}
	clientID := strings.TrimSpace(os.Getenv("SST_OIDC_CLIENT_ID"))
	if clientID == "" {
		return oidcConfig{}, fmt.Errorf("SST_OIDC_CLIENT_ID is not set; set it in .env.sst-cli (repo root or $HOME)")
	}
	clientSecret := os.Getenv("SST_OIDC_CLIENT_SECRET")
	if clientSecret == "" {
		return oidcConfig{}, fmt.Errorf("SST_OIDC_CLIENT_SECRET is not set; set it in .env.sst-cli (repo root or $HOME)")
	}
	return oidcConfig{
		realmURL:     realmURL,
		clientID:     clientID,
		clientSecret: clientSecret,
	}, nil
}

func GetToken(refreshToken string) *oauth2.Token {
	cfg, err := loadOIDCConfig()
	if err != nil {
		log.Print(err)
		return nil
	}
	realmURL := cfg.realmURL
	clientID := cfg.clientID
	secret := cfg.clientSecret
	username, password := GetUserCredentials()
	tokenURL := realmURL + keycloakOIDCTokenPath

	parms := url.Values{}
	parms.Add("client_id", clientID)
	if refreshToken == "" {
		parms.Add("grant_type", "password")
		parms.Add("username", username)
		parms.Add("password", password)
		parms.Add("client_secret", secret)
	} else {
		parms.Add("grant_type", "refresh_token")
		parms.Add("refresh_token", refreshToken)
	}

	response, err := http.PostForm(tokenURL, parms)
	if err != nil {
		log.Printf("Request Failed: %s", err)
		return &oauth2.Token{}
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		log.Printf("Failed to read response body: %s", err)
		return nil
	}

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		fmt.Println("JSON unmarshal failed:", err)
		return nil
	}

	accessToken := fmt.Sprint(data["access_token"])
	refreshToken = fmt.Sprint(data["refresh_token"])

	token := &oauth2.Token{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}

	if !VerifyTokenValidity(token.AccessToken) {
		fmt.Println("Token verification failed.")
		return nil
	}

	return token
}

type refreshableProvider struct {
	mu           sync.Mutex
	accessToken  string
	refreshToken string
}

func (p *refreshableProvider) AuthProvider() {}

func (p *refreshableProvider) Info() (email string, name string, err error) {
	return "", "", nil
}

func (p *refreshableProvider) Oauth2Token() (*oauth2.Token, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.accessToken == "" {
		tok := GetToken("")
		if tok == nil || tok.AccessToken == "" {
			return nil, fmt.Errorf("failed to authenticate (no access token)")
		}
		p.accessToken = tok.AccessToken
		p.refreshToken = tok.RefreshToken
	}

	// Refresh when the token is close to expiry.
	if !isAccessTokenValid(p.accessToken) {
		if p.refreshToken != "" {
			tok := GetToken(p.refreshToken)
			if tok == nil || tok.AccessToken == "" {
				return nil, fmt.Errorf("failed to refresh access token")
			}
			p.accessToken = tok.AccessToken
			p.refreshToken = tok.RefreshToken
		} else {
			tok := GetToken("")
			if tok == nil || tok.AccessToken == "" {
				return nil, fmt.Errorf("failed to re-authenticate (no refresh token)")
			}
			p.accessToken = tok.AccessToken
			p.refreshToken = tok.RefreshToken
		}
	}

	return &oauth2.Token{
		AccessToken:  p.accessToken,
		RefreshToken: p.refreshToken,
	}, nil
}

// GetRealProvider retrieves a valid authentication provider, prompting user if necessary
func GetRealProvider() *refreshableProvider {
	for {
		token := GetToken("")
		if token != nil {
			if token.AccessToken != "" {
				return &refreshableProvider{
					accessToken:  token.AccessToken,
					refreshToken: token.RefreshToken,
				}
			}
		}

		// Ask user if they want to retry
		fmt.Println("\nInvalid username/password. Press Enter to retry, or type 'q' to quit:")
		input, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		if strings.TrimSpace(input) == "q" {
			fmt.Println("Authentication cancelled.")
			os.Exit(1)
		}
	}
}

func isAccessTokenValid(accessToken string) bool {
	exp, ok := jwtExpUnix(accessToken)
	if !ok {
		return false
	}

	const refreshSkewSeconds = 10
	return time.Now().Unix() < exp-int64(refreshSkewSeconds)
}

func jwtExpUnix(jwt string) (int64, bool) {
	parts := strings.Split(jwt, ".")
	if len(parts) < 2 {
		return 0, false
	}

	// JWT payload is base64url-encoded JSON (no padding).
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return 0, false
	}

	var claims struct {
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return 0, false
	}
	if claims.Exp == 0 {
		return 0, false
	}
	return claims.Exp, true
}

// VerifyTokenValidity checks if the token is a valid JWT
func VerifyTokenValidity(accessToken string) bool {
	cfg, err := loadOIDCConfig()
	if err != nil {
		return false
	}
	provider, err := oidc.NewProvider(context.TODO(), cfg.realmURL)
	if err != nil {
		// fmt.Printf("OIDC Provider Error: %s\n", err)
		return false
	}

	verifier := provider.Verifier(&oidc.Config{SkipClientIDCheck: true})

	_, err = verifier.Verify(context.TODO(), accessToken)
	return err == nil
}

// GetAuthContext returns the appropriate context for a given repository.
// If the repository has an associated authentication context (e.g., for remote repositories),
// it returns that context. Otherwise, it returns context.TODO() as a fallback for local or unauthenticated use.
func GetAuthContext(repo sst.Repository, authContexts map[sst.Repository]context.Context) context.Context {
	if ctx, ok := authContexts[repo]; ok {
		return ctx
	}
	return context.TODO()
}

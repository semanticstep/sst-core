// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package mcp

import (
	"context"
	"fmt"

	"github.com/semanticstep/sst-core/cli/cmd/utils"
	"github.com/semanticstep/sst-core/sst"
	"github.com/semanticstep/sst-core/sstauth"
	"golang.org/x/oauth2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type staticTokenProvider struct {
	accessToken  string
	refreshToken string
}

func (p *staticTokenProvider) AuthProvider() {}

func (p *staticTokenProvider) Info() (string, string, error) {
	return "", "", nil
}

func (p *staticTokenProvider) Oauth2Token() (*oauth2.Token, error) {
	if p.accessToken == "" {
		return nil, fmt.Errorf("no access token")
	}
	return &oauth2.Token{
		AccessToken:  p.accessToken,
		RefreshToken: p.refreshToken,
	}, nil
}

func remoteAuthContext() (context.Context, error) {
	token := utils.GetToken("")
	if token == nil || token.AccessToken == "" {
		return nil, fmt.Errorf("authentication failed; set SST_OIDC_* and SST_USERNAME/SST_PASSWORD in .env.sst-cli")
	}
	provider := &staticTokenProvider{
		accessToken:  token.AccessToken,
		refreshToken: token.RefreshToken,
	}
	return sstauth.ContextWithAuthProvider(context.Background(), provider), nil
}

// OpenRemoteRepository opens a remote SST repository and registers it in the session.
func (s *Session) OpenRemoteRepository(url, alias string) (string, error) {
	if url == "" {
		return "", fmt.Errorf("url is required")
	}

	id, autoID, err := s.reserveRepoAlias(alias)
	if err != nil {
		return "", err
	}

	authCtx, err := remoteAuthContext()
	if err != nil {
		return "", err
	}

	creds := credentials.NewTLS(nil)

	var repo sst.Repository
	var panicErr any
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicErr = r
			}
		}()
		repo, err = sst.OpenRemoteRepository(authCtx, url, grpc.WithTransportCredentials(creds))
	}()
	if panicErr != nil {
		return "", fmt.Errorf("cannot connect to remote repository at %q: %v", url, panicErr)
	}
	if err != nil {
		msg, _ := utils.ExplainRemoteRepositoryOpenError(url, err)
		if msg != "" {
			return "", fmt.Errorf("%s", msg)
		}
		return "", err
	}
	if repo == nil {
		return "", fmt.Errorf("could not open remote repository %q (empty handle)", url)
	}

	s.commitRepo(id, autoID, url, "remote", repo, authCtx)
	return id, nil
}

func (s *Session) authContextFor(repo sst.Repository) context.Context {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ctx, ok := s.authContexts[repo]; ok {
		return ctx
	}
	return context.Background()
}

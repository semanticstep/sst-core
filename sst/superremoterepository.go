// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sst

import (
	"context"
	"fmt"
	"net/url"

	"github.com/semanticstep/sst-core/bboltproto"
	"github.com/semanticstep/sst-core/bleveproto"
	"github.com/semanticstep/sst-core/sstauth"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/oauth"
	"google.golang.org/grpc/status"
)

type remoteSuperRepository struct {
	url       *url.URL
	cc        *grpc.ClientConn
	mgrClient bboltproto.RepoManagerServiceClient
}

func OpenRemoteSuperRepository(
	ctx context.Context,
	URL string,
	tlsOption grpc.DialOption,
) (*remoteSuperRepository, error) {
	var dialOptions []grpc.DialOption
	if tlsOption != nil {
		dialOptions = append(dialOptions, tlsOption)
	}

	GlobalLogger.Info("Connecting to remote super repository", zap.String("target", URL))
	cc, err := grpc.NewClient(URL, dialOptions...)
	if err != nil {
		panic(err)
	}

	return &remoteSuperRepository{
		url:       &url.URL{Scheme: "", Host: URL},
		cc:        cc,
		mgrClient: bboltproto.NewRepoManagerServiceClient(cc),
	}, nil
}

func perRPCCallOpts(ctx context.Context) ([]grpc.CallOption, error) {
	var opts []grpc.CallOption
	if p, ok := sstauth.AuthProviderFromContext(ctx).(sstauth.Provider); ok {
		token, err := p.Oauth2Token()
		if err != nil {
			return nil, err
		}
		opts = append(opts, grpc.PerRPCCredentials(oauth.NewOauthAccess(token)))
	}
	return opts, nil
}

func (r *remoteSuperRepository) List(ctx context.Context) ([]string, error) {
	opts, err := perRPCCallOpts(ctx)
	if err != nil {
		return nil, err
	}

	resp, err := r.mgrClient.ListRepos(ctx, &bboltproto.ListReposRequest{}, opts...)
	if err != nil {
		return nil, err
	}
	return resp.Names, nil
}

func (r *remoteSuperRepository) Create(ctx context.Context, name string) (Repository, error) {
	opts, err := perRPCCallOpts(ctx)
	if err != nil {
		return nil, err
	}

	GlobalLogger.Info("Creating remote repository", zap.String("name", name), zap.String("target", r.url.String()))
	_, err = r.mgrClient.CreateRepo(ctx, &bboltproto.CreateRepoRequest{Name: name}, opts...)
	if err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.ResourceExhausted {
			return nil, fmt.Errorf("%w: %s", ErrQuotaExceeded, s.Message())
		}
		return nil, err
	}

	return r.Get(ctx, name)
}

func (r *remoteSuperRepository) Delete(ctx context.Context, name string) error {
	opts, err := perRPCCallOpts(ctx)
	if err != nil {
		return err
	}

	_, err = r.mgrClient.DeleteRepo(ctx, &bboltproto.DeleteRepoRequest{Name: name}, opts...)
	return err
}

func (r *remoteSuperRepository) Get(ctx context.Context, name string) (Repository, error) {
	if name == "" {
		name = "default"
	}

	repo := &remoteRepository{
		url:      &url.URL{Scheme: r.url.Scheme, Host: r.url.Host, Fragment: name},
		cc:       r.cc,
		repoName: name,
		sr:       r,

		dsClient:     bboltproto.NewDatasetServiceClient(r.cc),
		commitClient: bboltproto.NewCommitServiceClient(r.cc),
		reqCache:     newRemoteRequestCache(),
		remoteIndexIns: remoteIndex{
			repoName: name,
			idx:      bleveproto.NewIndexServiceClient(r.cc),
		},
	}
	repo.state.Store(int32(stateOpen))

	return repo, nil
}

// URL returns the grpc:// URL for this remote SuperRepository.
func (r *remoteSuperRepository) URL() string {
	return r.url.String()
}

func (r *remoteSuperRepository) RegisterIndexHandler(*SSTDeriveInfo) error {
	return ErrNotSupported
}

func (r *remoteSuperRepository) GetQuota(ctx context.Context, name string) (RepositoryQuota, error) {
	opts, err := perRPCCallOpts(ctx)
	if err != nil {
		return RepositoryQuota{}, err
	}
	resp, err := r.mgrClient.GetRepoQuota(ctx, &bboltproto.GetRepoQuotaRequest{Name: name}, opts...)
	if err != nil {
		return RepositoryQuota{}, err
	}
	return RepositoryQuota{MaxSizeBytes: resp.GetMaxSizeBytes(), ActualSizeBytes: resp.GetActualSizeBytes()}, nil
}

func (r *remoteSuperRepository) SetQuota(ctx context.Context, name string, maxSizeBytes int64) error {
	opts, err := perRPCCallOpts(ctx)
	if err != nil {
		return err
	}
	_, err = r.mgrClient.SetRepoQuota(ctx, &bboltproto.SetRepoQuotaRequest{Name: name, MaxSizeBytes: maxSizeBytes}, opts...)
	return err
}

func (r *remoteSuperRepository) GetTotalQuota(ctx context.Context) (RepositoryQuota, error) {
	opts, err := perRPCCallOpts(ctx)
	if err != nil {
		return RepositoryQuota{}, err
	}
	resp, err := r.mgrClient.GetSuperQuota(ctx, &bboltproto.GetSuperQuotaRequest{}, opts...)
	if err != nil {
		return RepositoryQuota{}, err
	}
	return RepositoryQuota{MaxSizeBytes: resp.GetMaxSizeBytes(), ActualSizeBytes: resp.GetActualSizeBytes()}, nil
}

func (r *remoteSuperRepository) SetTotalQuota(ctx context.Context, maxSizeBytes int64) error {
	opts, err := perRPCCallOpts(ctx)
	if err != nil {
		return err
	}
	_, err = r.mgrClient.SetSuperQuota(ctx, &bboltproto.SetSuperQuotaRequest{MaxSizeBytes: maxSizeBytes}, opts...)
	return err
}

func (r *remoteSuperRepository) GetMaxRepositoryCount(ctx context.Context) int {
	opts, err := perRPCCallOpts(ctx)
	if err != nil {
		return 0
	}
	resp, err := r.mgrClient.GetMaxRepoCount(ctx, &bboltproto.GetMaxRepoCountRequest{}, opts...)
	if err != nil {
		return 0
	}
	return int(resp.GetCount())
}

func (r *remoteSuperRepository) SetMaxRepositoryCount(ctx context.Context, count int) error {
	opts, err := perRPCCallOpts(ctx)
	if err != nil {
		return err
	}
	_, err = r.mgrClient.SetMaxRepoCount(ctx, &bboltproto.SetMaxRepoCountRequest{Count: int32(count)}, opts...)
	return err
}

// Close closes the gRPC connection to the remote super repository.
func (r *remoteSuperRepository) Close() error {
	if r.cc != nil {
		return r.cc.Close()
	}
	return nil
}

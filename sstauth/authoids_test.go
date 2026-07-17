// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sstauth

import (
	"context"
	"testing"

	"github.com/semanticstep/sst-core/bboltproto"
	"github.com/semanticstep/sst-core/bleveproto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepoNameFromRequest(t *testing.T) {
	tests := []struct {
		name     string
		req      any
		expected string
	}{
		{
			name:     "SetBranchRequest with repo name",
			req:      &bboltproto.SetBranchRequest{RepoName: "sales"},
			expected: "sales",
		},
		{
			name:     "ListDatasetsRequest with repo name",
			req:      &bboltproto.ListDatasetsRequest{RepoName: "engineering"},
			expected: "engineering",
		},
		{
			name:     "SearchRequest with repo name",
			req:      &bleveproto.SearchRequest{RepoName: "marketing"},
			expected: "marketing",
		},
		{
			name:     "empty repo name falls back to default",
			req:      &bboltproto.SetBranchRequest{RepoName: ""},
			expected: "default",
		},
		{
			name:     "request without RepoName falls back to default",
			req:      &bboltproto.GetBranchesResponse{},
			expected: "default",
		},
		{
			name:     "nil request falls back to default",
			req:      nil,
			expected: "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, RepoNameFromRequest(tt.req))
		})
	}
}

func TestAccessModeForClientID(t *testing.T) {
	claims := KCClaims{
		ResourceAccess: map[string]struct {
			Roles []string `json:"roles"`
		}{
			"grpc://example.com#sales": {Roles: []string{"ReadWrite"}},
			"grpc://example.com#hr":    {Roles: []string{"ReadOnly"}},
			"grpc://example.com#admin": {Roles: []string{"Admin"}},
		},
	}

	tests := []struct {
		clientID string
		expected AccessMode
	}{
		{"grpc://example.com#sales", AccessMode_ReadWrite},
		{"grpc://example.com#hr", AccessMode_ReadOnly},
		{"grpc://example.com#admin", AccessMode_Admin},
		{"grpc://example.com#unknown", AccessMode_None},
	}

	for _, tt := range tests {
		t.Run(tt.clientID, func(t *testing.T) {
			assert.Equal(t, tt.expected, AccessModeForClientID(claims, tt.clientID))
		})
	}
}

func TestCheckRepoAccess(t *testing.T) {
	methodRoles := map[string]AccessMode{
		"/sst.repository.DatasetService/SetBranch":  AccessMode_ReadWrite,
		"/sst.repository.DatasetService/GetDataset": AccessMode_ReadOnly,
	}

	superClientID := "grpc://example.com"
	repoURL := superClientID + "#sales"

	tests := []struct {
		name           string
		claims         KCClaims
		audience       []string
		repoURL        string
		fullMethod     string
		useSuperClient bool
		expectErr      bool
	}{
		{
			name:       "read access on read method allowed",
			claims:     KCClaims{ResourceAccess: resourceAccessFor("grpc://example.com#sales", "ReadOnly")},
			audience:   []string{"grpc://example.com#sales"},
			repoURL:    repoURL,
			fullMethod: "/sst.repository.DatasetService/GetDataset",
			expectErr:  false,
		},
		{
			name:       "read access on write method denied",
			claims:     KCClaims{ResourceAccess: resourceAccessFor("grpc://example.com#sales", "ReadOnly")},
			audience:   []string{"grpc://example.com#sales"},
			repoURL:    repoURL,
			fullMethod: "/sst.repository.DatasetService/SetBranch",
			expectErr:  true,
		},
		{
			name:       "write access on write method allowed",
			claims:     KCClaims{ResourceAccess: resourceAccessFor("grpc://example.com#sales", "ReadWrite")},
			audience:   []string{"grpc://example.com#sales"},
			repoURL:    repoURL,
			fullMethod: "/sst.repository.DatasetService/SetBranch",
			expectErr:  false,
		},
		{
			name:       "wrong repo client denied",
			claims:     KCClaims{ResourceAccess: resourceAccessFor("grpc://example.com#hr", "ReadWrite")},
			audience:   []string{"grpc://example.com#hr"},
			repoURL:    repoURL,
			fullMethod: "/sst.repository.DatasetService/SetBranch",
			expectErr:  true,
		},
		{
			name:           "super client method uses super client ID",
			claims:         KCClaims{ResourceAccess: resourceAccessFor("grpc://example.com", "SuperAdmin")},
			audience:       []string{"grpc://example.com"},
			repoURL:        repoURL,
			fullMethod:     "/sst.repository.RepoManagerService/ListRepos",
			useSuperClient: true,
			expectErr:      false,
		},
		{
			name:       "missing repo audience denied",
			claims:     KCClaims{ResourceAccess: resourceAccessFor("grpc://example.com#sales", "ReadWrite")},
			audience:   []string{"grpc://example.com"},
			repoURL:    repoURL,
			fullMethod: "/sst.repository.DatasetService/SetBranch",
			expectErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := WithPrincipal(context.Background(), &Principal{Claims: tt.claims, Audience: tt.audience})
			err := CheckRepoAccess(ctx, tt.repoURL, tt.fullMethod, methodRoles, superClientID, tt.useSuperClient)
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCheckRepoAccessMissingPrincipal(t *testing.T) {
	err := CheckRepoAccess(
		context.Background(),
		"grpc://example.com#sales",
		"/sst.repository.DatasetService/GetDataset",
		map[string]AccessMode{"/sst.repository.DatasetService/GetDataset": AccessMode_ReadOnly},
		"grpc://example.com",
		false,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing principal")
}

func resourceAccessFor(clientID string, roles ...string) map[string]struct {
	Roles []string `json:"roles"`
} {
	return map[string]struct {
		Roles []string `json:"roles"`
	}{
		clientID: {Roles: roles},
	}
}

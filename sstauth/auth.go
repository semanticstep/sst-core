// Copyright Semantic STEP Technology GmbH, Germany & DCT Co., Ltd. Tianjin, China

package sstauth

import (
	"context"
)

// AuthProvider defines an interface for authentication providers.
// It requires two methods:
//   - AuthProvider: A method to implement the authentication logic.
//   - Info: A method to retrieve the email and name of the authenticated user,
//     returning an error if the information cannot be retrieved.
type AuthProvider interface {
	AuthProvider()
	Info() (email string, name string, err error)
}

// SstUserInfo identifies who is using the Repository by email and name.
// This data is used to create a commit.
type SstUserInfo struct {
	Email string
}

func ContextWithSstUserInfo(ctx context.Context, info *SstUserInfo) context.Context {
	return context.WithValue(ctx, SstUserInfo{}, info)
}

// SstUserInfoFromContext extract [sstUserInfo] from context.
func SstUserInfoFromContext(ctx context.Context) *SstUserInfo {
	authInfo := ctx.Value(SstUserInfo{})
	if authInfo, ok := authInfo.(*SstUserInfo); ok {
		return authInfo
	}
	return nil
}

type authenticationProvider struct{}

// takes an input context and returns an enhanced context with authenticationProvider.
func ContextWithAuthProvider(ctx context.Context, provider AuthProvider) context.Context {
	return context.WithValue(ctx, authenticationProvider{}, provider)
}

// takes an input context and extract the authenticationProvider out.
func AuthProviderFromContext(ctx context.Context) AuthProvider {
	provider := ctx.Value(authenticationProvider{})
	if provider, ok := provider.(AuthProvider); ok {
		return provider
	}
	return nil
}

// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fnapi

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"buf.build/gen/go/namespace/cloud/connectrpc/go/proto/namespace/cloud/github/v1beta/githubv1betaconnect"
	"connectrpc.com/connect"
	"namespacelabs.dev/foundation/internal/versions"
	"namespacelabs.dev/integrations/api"
)

func newAuthInterceptor(tokenSource api.TokenSource) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			token, err := tokenSource.IssueToken(ctx, 15*time.Minute, false)
			if err != nil {
				return nil, err
			}
			req.Header().Set("Authorization", "Bearer "+token)
			req.Header().Set("NS-Internal-Version", fmt.Sprintf("%d", versions.Builtin().APIVersion))
			req.Header().Set("User-Agent", UserAgent)
			if AdminMode {
				req.Header().Set("NS-API-Mode", "admin")
			}
			return next(ctx, req)
		}
	}
}

// NewProfileServiceClient creates a new ProfileService client with authentication.
func NewProfileServiceClient(ctx context.Context) (githubv1betaconnect.ProfileServiceClient, error) {
	tok, err := FetchToken(ctx)
	if err != nil {
		return nil, err
	}

	endpoint, err := ResolveGlobalEndpoint(ctx, ResolvedToken{})
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{
		Transport: http.DefaultTransport,
	}

	// Convert Token to api.TokenSource
	tokenSource := &tokenSourceAdapter{token: tok}

	client := githubv1betaconnect.NewProfileServiceClient(
		httpClient,
		endpoint,
		connect.WithInterceptors(newAuthInterceptor(tokenSource)),
	)

	return client, nil
}

// tokenSourceAdapter adapts fnapi.Token to api.TokenSource
type tokenSourceAdapter struct {
	token Token
}

func (t *tokenSourceAdapter) IssueToken(ctx context.Context, minDuration time.Duration, force bool) (string, error) {
	return t.token.IssueToken(ctx, minDuration, force)
}

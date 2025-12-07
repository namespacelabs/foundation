// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fnapi

import (
	"context"
	"net/http"
	"time"

	"buf.build/gen/go/namespace/cloud/connectrpc/go/proto/namespace/cloud/github/v1beta/githubv1betaconnect"
	"connectrpc.com/connect"
	"namespacelabs.dev/integrations/api"
)

func newAuthInterceptor(tokenSource api.TokenSource) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			token, err := tokenSource.IssueToken(ctx, 15*time.Minute, false)
			if err != nil {
				return nil, err
			}

			AddNamespaceHeaders(req.Header())
			req.Header().Set("Authorization", "Bearer "+token)

			return next(ctx, req)
		}
	}
}

func NewProfileServiceClient(ctx context.Context) (githubv1betaconnect.ProfileServiceClient, error) {
	tok, err := FetchToken(ctx)
	if err != nil {
		return nil, err
	}

	client := githubv1betaconnect.NewProfileServiceClient(
		http.DefaultClient,
		GlobalEndpoint(),
		connect.WithInterceptors(newAuthInterceptor(tok)),
	)

	return client, nil
}

func NewJobsServiceClient(ctx context.Context) (githubv1betaconnect.JobsServiceClient, error) {
	tok, err := FetchToken(ctx)
	if err != nil {
		return nil, err
	}

	client := githubv1betaconnect.NewJobsServiceClient(
		http.DefaultClient,
		GlobalEndpoint(),
		connect.WithInterceptors(newAuthInterceptor(tok)),
	)

	return client, nil
}

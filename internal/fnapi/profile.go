// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fnapi

import (
	"context"
	"net/http"
	"time"

	"connectrpc.com/connect"
	"namespacelabs.dev/integrations/api"
	"namespacelabs.dev/integrations/proto/namespace/cloud/github/v1beta/githubv1betaconnect"
)

type authInterceptor struct {
	tokenSource api.TokenSource
}

func newAuthInterceptor(tokenSource api.TokenSource) connect.Interceptor {
	return authInterceptor{tokenSource: tokenSource}
}

func (a authInterceptor) setHeaders(ctx context.Context, headers http.Header) error {
	token, err := a.tokenSource.IssueToken(ctx, 15*time.Minute, false)
	if err != nil {
		return err
	}

	AddNamespaceHeaders(headers)
	headers.Set("Authorization", "Bearer "+token)
	return nil
}

func (a authInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		if err := a.setHeaders(ctx, req.Header()); err != nil {
			return nil, err
		}

		return next(ctx, req)
	}
}

func (a authInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return func(ctx context.Context, spec connect.Spec) connect.StreamingClientConn {
		conn := next(ctx, spec)
		if err := a.setHeaders(ctx, conn.RequestHeader()); err != nil {
			// StreamingClientFunc cannot fail, so surface the error on the stream's first use.
			return failedStreamingClientConn{StreamingClientConn: conn, err: err}
		}

		return conn
	}
}

func (a authInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next
}

type failedStreamingClientConn struct {
	connect.StreamingClientConn
	err error
}

func (c failedStreamingClientConn) Send(any) error    { return c.err }
func (c failedStreamingClientConn) Receive(any) error { return c.err }

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

func NewImageServiceClient(ctx context.Context) (githubv1betaconnect.ImageServiceClient, error) {
	tok, err := FetchToken(ctx)
	if err != nil {
		return nil, err
	}

	client := githubv1betaconnect.NewImageServiceClient(
		http.DefaultClient,
		GlobalEndpoint(),
		connect.WithInterceptors(newAuthInterceptor(tok)),
	)

	return client, nil
}

// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fnapi

import (
	"context"
	"os"
	"time"

	"namespacelabs.dev/foundation/internal/auth"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/std/tasks"
)

const AdminScope = "admin"

type ExchangeGithubTokenRequest struct {
	GithubToken string `json:"github_token,omitempty"`
}

type ExchangeGithubTokenResponse struct {
	TenantToken string  `json:"tenant_token,omitempty"`
	Tenant      *Tenant `json:"tenant,omitempty"`
}

type ExchangeCircleciTokenRequest struct {
	CircleciToken string `json:"circleci_token,omitempty"`
}

type ExchangeCircleciTokenResponse struct {
	TenantToken string  `json:"tenant_token,omitempty"`
	Tenant      *Tenant `json:"tenant,omitempty"`
}

type Tenant struct {
	Name   string `json:"name,omitempty"`
	AppUrl string `json:"app_url,omitempty"`
}

func ExchangeGithubToken(ctx context.Context, jwt string) (ExchangeGithubTokenResponse, error) {
	req := ExchangeGithubTokenRequest{GithubToken: jwt}

	var res ExchangeGithubTokenResponse
	if err := AnonymousCall(ctx, EndpointAddress, "nsl.tenants.TenantsService/ExchangeGithubToken", req, DecodeJSONResponse(&res)); err != nil {
		return ExchangeGithubTokenResponse{}, err
	}

	return res, nil
}

func ExchangeCircleciToken(ctx context.Context, token string) (ExchangeCircleciTokenResponse, error) {
	req := ExchangeCircleciTokenRequest{CircleciToken: token}

	var res ExchangeCircleciTokenResponse
	if err := AnonymousCall(ctx, EndpointAddress, "nsl.tenants.TenantsService/ExchangeCircleciToken", req, DecodeJSONResponse(&res)); err != nil {
		return ExchangeCircleciTokenResponse{}, err
	}

	return res, nil
}

type userToken string

func (u userToken) Raw() string {
	return string(u)
}

type ExchangeUserTokenRequest struct {
	Scopes []string `json:"scopes,omitempty"`
}

type ExchangeUserTokenResponse struct {
	TenantToken string `json:"tenant_token,omitempty"`
}

func ExchangeUserToken(ctx context.Context, token string, scopes ...string) (ExchangeUserTokenResponse, error) {
	req := ExchangeUserTokenRequest{Scopes: scopes}

	var res ExchangeUserTokenResponse
	if err := (Call[ExchangeUserTokenRequest]{
		Method: "nsl.tenants.TenantsService/ExchangeUserToken",
		FetchToken: func(ctx context.Context) (Token, error) {
			return userToken(token), nil
		},
	}.Do(ctx, req, ResolveStaticEndpoint(EndpointAddress), DecodeJSONResponse(&res))); err != nil {
		return ExchangeUserTokenResponse{}, err
	}

	return res, nil
}

type ExchangeTenantTokenRequest struct {
	TenantToken string   `json:"tenant_token,omitempty"`
	Scopes      []string `json:"scopes,omitempty"`
}

type ExchangeTenantTokenResponse struct {
	TenantToken string `json:"tenant_token,omitempty"`
}

func ExchangeTenantToken(ctx context.Context, scopes []string) (ExchangeTenantTokenResponse, error) {
	req := ExchangeTenantTokenRequest{Scopes: scopes}

	var res ExchangeTenantTokenResponse
	if err := AuthenticatedCall(ctx, EndpointAddress, "nsl.tenants.TenantsService/ExchangeTenantToken", req, DecodeJSONResponse(&res)); err != nil {
		return ExchangeTenantTokenResponse{}, err
	}

	return res, nil
}

func ResolveSpec() (string, error) {
	if spec := os.Getenv("NSC_TOKEN_SPEC"); spec != "" {
		return spec, nil
	}

	if specFile := os.Getenv("NSC_TOKEN_SPEC_FILE"); specFile != "" {
		contents, err := os.ReadFile(specFile)
		if err != nil {
			return "", fnerrors.New("failed to load spec: %w", err)
		}

		return string(contents), nil
	}

	return "", nil
}

func FetchToken(ctx context.Context) (Token, error) {
	return tasks.Return(ctx, tasks.Action("nsc.fetch-token").LogLevel(1), func(ctx context.Context) (*auth.Token, error) {
		spec, err := ResolveSpec()
		if err != nil {
			return nil, err
		}

		if spec != "" {
			return auth.FetchTokenFromSpec(ctx, spec)
		}

		if specified := os.Getenv("NSC_TOKEN_FILE"); specified != "" {
			return auth.LoadTokenFromPath(ctx, specified, time.Now())
		}

		return auth.LoadTenantToken(ctx)
	})
}

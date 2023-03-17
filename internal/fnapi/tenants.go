// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fnapi

import (
	"context"
	"os"

	"namespacelabs.dev/foundation/internal/auth"
	"namespacelabs.dev/foundation/std/tasks"
)

const AdminScope = "admin"

type ExchangeGithubTokenRequest struct {
	GithubToken string `json:"github_token,omitempty"`
}

type ExchangeGithubTokenResponse struct {
	TenantToken string `json:"tenant_token,omitempty"`
	UserError   string `json:"user_error,omitempty"`
}

func ExchangeGithubToken(ctx context.Context, jwt string) (ExchangeGithubTokenResponse, error) {
	req := ExchangeGithubTokenRequest{GithubToken: jwt}

	var res ExchangeGithubTokenResponse
	if err := AnonymousCall(ctx, EndpointAddress, "nsl.tenants.TenantsService/ExchangeGithubToken", req, DecodeJSONResponse(&res)); err != nil {
		return ExchangeGithubTokenResponse{}, err
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
		Endpoint: EndpointAddress,
		Method:   "nsl.tenants.TenantsService/ExchangeUserToken",
		FetchToken: func(ctx context.Context) (Token, error) {
			return userToken(token), nil
		},
	}.Do(ctx, req, DecodeJSONResponse(&res))); err != nil {
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

func FetchTenantToken(ctx context.Context) (Token, error) {
	return tasks.Return(ctx, tasks.Action("tenants.fetch-tenant-token"), func(ctx context.Context) (*auth.Token, error) {
		if AdminMode {
			return auth.LoadAdminToken(ctx)
		}

		if specified := os.Getenv("NSC_TOKEN_FILE"); specified != "" {
			return auth.LoadTokenFromPath(ctx, specified)
		}

		return auth.LoadTenantToken(ctx)
	})
}

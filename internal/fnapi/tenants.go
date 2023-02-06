// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fnapi

import (
	"context"
)

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

type ExchangeUserTokenRequest struct {
	Token  string   `json:"token,omitempty"`
	Scopes []string `json:"scopes,omitempty"`
}

type ExchangeUserTokenResponse struct {
	TenantToken string `json:"tenant_token,omitempty"`
}

func ExchangeUserToken(ctx context.Context, token string, scopes []string) (ExchangeUserTokenResponse, error) {
	req := ExchangeUserTokenRequest{Token: token, Scopes: scopes}

	var res ExchangeUserTokenResponse
	if err := AnonymousCall(ctx, EndpointAddress, "nsl.tenants.TenantsService/ExchangeUserToken", req, DecodeJSONResponse(&res)); err != nil {
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

func ExchangeTenantToken(ctx context.Context, token string, scopes []string) (ExchangeTenantTokenResponse, error) {
	req := ExchangeTenantTokenRequest{TenantToken: token, Scopes: scopes}

	var res ExchangeTenantTokenResponse
	if err := AnonymousCall(ctx, EndpointAddress, "nsl.tenants.TenantsService/ExchangeTenantToken", req, DecodeJSONResponse(&res)); err != nil {
		return ExchangeTenantTokenResponse{}, err
	}

	return res, nil
}

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
}

func ExchangeGithubToken(ctx context.Context, jwt string) (string, error) {
	req := ExchangeGithubTokenRequest{GithubToken: jwt}

	var resp ExchangeGithubTokenResponse
	if err := AnonymousCall(ctx, EndpointAddress, "nsl.tenants.TenantsService/ExchangeGithubToken", req, DecodeJSONResponse(&resp)); err != nil {
		return "", err
	}

	return resp.TenantToken, nil
}

type BlockTenantRequest struct {
	TenantId string `json:"tenant_id,omitempty"`
}

func BlockTenant(ctx context.Context, id string) error {
	req := BlockTenantRequest{TenantId: id}

	return AuthenticatedCall(ctx, EndpointAddress, "nsl.tenants.TenantsService/BlockTenant", req, nil)
}

type UnblockTenantRequest struct {
	TenantId string `json:"tenant_id,omitempty"`
}

func UnblockTenant(ctx context.Context, id string) error {
	req := BlockTenantRequest{TenantId: id}

	return AuthenticatedCall(ctx, EndpointAddress, "nsl.tenants.TenantsService/UnblockTenant", req, nil)
}

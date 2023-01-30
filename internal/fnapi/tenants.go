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

type ExchangeGithubTokenResponse_UserError int32

const (
	ExchangeGithubTokenResponse_USER_ERROR_UNKNOWN ExchangeGithubTokenResponse_UserError = 0
	ExchangeGithubTokenResponse_NO_INSTALLATION    ExchangeGithubTokenResponse_UserError = 1
)

type ExchangeGithubTokenResponse struct {
	TenantToken string                                `json:"tenant_token,omitempty"`
	UserError   ExchangeGithubTokenResponse_UserError `json:"user_error,omitempty"`
}

func ExchangeGithubToken(ctx context.Context, jwt string) (ExchangeGithubTokenResponse, error) {
	req := ExchangeGithubTokenRequest{GithubToken: jwt}

	var res ExchangeGithubTokenResponse
	if err := AnonymousCall(ctx, EndpointAddress, "nsl.tenants.TenantsService/ExchangeGithubToken", req, DecodeJSONResponse(&res)); err != nil {
		return ExchangeGithubTokenResponse{}, err
	}

	return res, nil
}

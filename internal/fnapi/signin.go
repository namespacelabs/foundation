// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fnapi

import (
	"context"
	"time"

	"namespacelabs.dev/foundation/internal/fnerrors"
)

type StartLoginRequest struct {
	Kind                string   `json:"kind"`
	SupportedKinds      []string `json:"supported_kinds"`
	TenantId            string   `json:"tenant_id,omitempty"`
	SessionDurationSecs int64    `json:"session_duration_secs,omitempty"`
}

type StartLoginResponse struct {
	LoginId  string `json:"login_id"`
	LoginUrl string `json:"login_url"`
	Kind     string `json:"kind"`
}

type CompleteLoginRequest struct {
	LoginId string `json:"login_id"`
}

type IssueTenantTokenFromSessionRequest struct {
	SessionToken      string `json:"session_token,omitempty"`
	TokenDurationSecs int64  `json:"token_duration_secs,omitempty"`
}

type IssueTenantTokenFromSessionResponse struct {
	TenantToken string `json:"tenant_token,omitempty"`
}

// Returns the URL which the user should open.
func StartLogin(ctx context.Context, tenantId string, sessionDuration time.Duration) (*StartLoginResponse, error) {
	req := StartLoginRequest{
		SupportedKinds:      []string{"tenant"},
		TenantId:            tenantId,
		SessionDurationSecs: int64(sessionDuration.Seconds()),
	}

	var resp StartLoginResponse
	if err := AnonymousCall(ctx, ResolveIAMEndpoint, "nsl.signin.SigninService/StartLogin", req, DecodeJSONResponse(&resp)); err != nil {
		return nil, err
	}

	if resp.LoginUrl == "" {
		return nil, fnerrors.InternalError("bad login response")
	}

	if resp.Kind != "tenant" {
		return nil, fnerrors.InternalError("unexpected kind %q", resp.Kind)
	}

	return &resp, nil
}

type CompleteTenantLoginResponse struct {
	TenantToken  string `json:"tenant_token,omitempty"`
	TenantName   string `json:"tenant_name,omitempty"`
	SessionToken string `json:"session_token,omitempty"`
}

func CompleteTenantLogin(ctx context.Context, id string) (*CompleteTenantLoginResponse, error) {
	req := CompleteLoginRequest{
		LoginId: id,
	}

	method := "nsl.signin.SigninService/CompleteTenantLogin"

	var resp []CompleteTenantLoginResponse
	// Explicitly use CallAPI() so we don't surface an action to the user while waiting.
	if err := AnonymousCall(ctx, ResolveIAMEndpoint, method, req, DecodeJSONResponse(&resp)); err != nil {
		return nil, err
	}

	if len(resp) != 1 {
		return nil, fnerrors.InternalError("expected exactly one response (got %d)", len(resp))
	}

	return &resp[0], nil
}

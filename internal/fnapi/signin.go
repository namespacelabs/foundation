// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fnapi

import (
	"context"

	"namespacelabs.dev/foundation/internal/auth"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

type StartLoginRequest struct {
	Kind              string   `json:"kind"`
	SupportedKinds    []string `json:"supported_kinds"`
	ImpersonateTenant string   `json:"impersonate_tenant,omitempty"`
}

type StartLoginResponse struct {
	LoginId  string `json:"login_id"`
	LoginUrl string `json:"login_url"`
	Kind     string `json:"kind"`
}

type CompleteLoginRequest struct {
	LoginId        string `json:"login_id"`
	EphemeralCliId string `json:"ephemeral_cli_id"`
}

// Returns the URL which the user should open.
func StartLogin(ctx context.Context, kind, impersonateTenant string) (*StartLoginResponse, error) {
	req := StartLoginRequest{Kind: kind, SupportedKinds: []string{"clerk", "tenant"}}

	if impersonateTenant != "" {
		if AdminMode {
			req.ImpersonateTenant = impersonateTenant
		} else {
			return nil, fnerrors.UsageError("specify --fnapi_admin to impersonate", "admin mode required")
		}
	}

	var resp StartLoginResponse
	if err := AnonymousCall(ctx, EndpointAddress, "nsl.signin.SigninService/StartLogin", req, DecodeJSONResponse(&resp)); err != nil {
		return nil, err
	}

	if resp.LoginUrl == "" {
		return nil, fnerrors.InternalError("bad login response")
	}

	return &resp, nil
}

func CompleteLogin(ctx context.Context, id, ephemeralCliId string) (*auth.UserAuth, error) {
	req := CompleteLoginRequest{
		LoginId:        id,
		EphemeralCliId: ephemeralCliId,
	}

	method := "nsl.signin.SigninService/CompleteLogin"

	var resp []auth.UserAuth
	// Explicitly use CallAPI() so we don't surface an action to the user while waiting.
	if err := AnonymousCall(ctx, EndpointAddress, method, req, DecodeJSONResponse(&resp)); err != nil {
		return nil, err
	}

	if len(resp) != 1 {
		return nil, fnerrors.InternalError("expected exactly one response (got %d)", len(resp))
	}

	return &resp[0], nil
}

type CompleteClerkLoginResponse struct {
	Ticket string `json:"ticket,omitempty"`
}

func CompleteClerkLogin(ctx context.Context, id, ephemeralCliId string) (*CompleteClerkLoginResponse, error) {
	req := CompleteLoginRequest{
		LoginId:        id,
		EphemeralCliId: ephemeralCliId,
	}

	method := "nsl.signin.SigninService/CompleteClerkLogin"

	var resp []CompleteClerkLoginResponse
	// Explicitly use CallAPI() so we don't surface an action to the user while waiting.
	if err := AnonymousCall(ctx, EndpointAddress, method, req, DecodeJSONResponse(&resp)); err != nil {
		return nil, err
	}

	if len(resp) != 1 {
		return nil, fnerrors.InternalError("expected exactly one response (got %d)", len(resp))
	}

	return &resp[0], nil
}

type CompleteTenantLoginResponse struct {
	TenantToken string `json:"tenant_token,omitempty"`
	TenantName  string `json:"tenant_name,omitempty"`
}

func CompleteTenantLogin(ctx context.Context, id, ephemeralCliId string) (*CompleteTenantLoginResponse, error) {
	req := CompleteLoginRequest{
		LoginId:        id,
		EphemeralCliId: ephemeralCliId,
	}

	method := "nsl.signin.SigninService/CompleteTenantLogin"

	var resp []CompleteTenantLoginResponse
	// Explicitly use CallAPI() so we don't surface an action to the user while waiting.
	if err := AnonymousCall(ctx, EndpointAddress, method, req, DecodeJSONResponse(&resp)); err != nil {
		return nil, err
	}

	if len(resp) != 1 {
		return nil, fnerrors.InternalError("expected exactly one response (got %d)", len(resp))
	}

	return &resp[0], nil
}

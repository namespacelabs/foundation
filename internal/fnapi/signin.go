// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fnapi

import (
	"context"
	"time"

	"namespacelabs.dev/foundation/internal/clerk"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

type StartLoginRequest struct {
	Kind string `json:"kind"`
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

type CheckRequest struct {
	UserData string `json:"userData"`
}

type RobotLoginRequest struct {
	Repository  string `json:"repository"`
	AccessToken string `json:"accessToken"`
}

type GetSessionTokenRequest struct {
	UserData        string `json:"user_data"`
	DurationSeconds uint32 `json:"duration_seconds"`
}

type GetSessionTokenResponse struct {
	Token      string    `json:"token"`
	Expiration time.Time `json:"expiration"`
}

// Returns the URL which the user should open.
func StartLogin(ctx context.Context, kind string) (*StartLoginResponse, error) {
	req := StartLoginRequest{Kind: kind}

	var resp StartLoginResponse
	if err := AnonymousCall(ctx, EndpointAddress, "nsl.signin.SigninService/StartLogin", req, DecodeJSONResponse(&resp)); err != nil {
		return nil, err
	}

	if resp.LoginUrl == "" {
		return nil, fnerrors.InternalError("bad login response")
	}

	return &resp, nil
}

func CompleteLogin(ctx context.Context, id, kind string, ephemeralCliId string) (*UserAuth, error) {
	if kind == "clerk" {
		t, err := completeClerkLogin(ctx, id, ephemeralCliId)
		if err != nil {
			return nil, err
		}

		n, err := clerk.Login(ctx, t.Ticket)
		if err != nil {
			return nil, err
		}

		return &UserAuth{
			Username: n.Email,
			Clerk:    n,
		}, nil
	}

	req := CompleteLoginRequest{
		LoginId:        id,
		EphemeralCliId: ephemeralCliId,
	}

	method := "nsl.signin.SigninService/CompleteLogin"

	var resp []UserAuth
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

func completeClerkLogin(ctx context.Context, id string, ephemeralCliId string) (*CompleteClerkLoginResponse, error) {
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

func CheckSignin(ctx context.Context, userData string) (*UserAuth, error) {
	req := CheckRequest{
		UserData: userData,
	}

	userAuth := &UserAuth{}
	err := AnonymousCall(ctx, EndpointAddress, "nsl.signin.SigninService/Check", req, DecodeJSONResponse(userAuth))
	return userAuth, err
}

func RobotLogin(ctx context.Context, repository, accessToken string) (*UserAuth, error) {
	req := RobotLoginRequest{
		Repository:  repository,
		AccessToken: accessToken,
	}

	userAuth := &UserAuth{}
	err := AnonymousCall(ctx, EndpointAddress, "nsl.signin.SigninService/RobotLogin", req, DecodeJSONResponse(userAuth))
	return userAuth, err
}

// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package fnapi

import (
	"context"

	"namespacelabs.dev/foundation/internal/fnerrors"
)

type StartLoginRequest struct{}

type StartLoginResponse struct {
	LoginId string `json:"login_id"`
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

func StartLogin(ctx context.Context) (string, error) {
	req := StartLoginRequest{}

	resp := &StartLoginResponse{}
	err := AnonymousCall(ctx, EndpointAddress, "nsl.signin.SigninService/StartLogin", req, DecodeJSONResponse(resp))

	return resp.LoginId, err
}

func CompleteLogin(ctx context.Context, id string, ephemeralCliId string) (*UserAuth, error) {
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

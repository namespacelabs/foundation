// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package fnapi

import (
	"context"
	"encoding/json"
)

type StartLoginRequest struct{}

type StartLoginResponse struct {
	LoginId string `json:"login_id"`
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
	err := callProdAPI(ctx, "nsl.signin.SigninService/StartLogin", req, func(dec *json.Decoder) error {
		return dec.Decode(resp)
	})

	return resp.LoginId, err
}

func CheckSignin(ctx context.Context, userData string) (*UserAuth, error) {
	req := CheckRequest{
		UserData: userData,
	}

	userAuth := &UserAuth{}
	err := callProdAPI(ctx, "nsl.signin.SigninService/Check", req, func(dec *json.Decoder) error {
		return dec.Decode(userAuth)
	})
	return userAuth, err
}

func RobotLogin(ctx context.Context, repository, accessToken string) (*UserAuth, error) {
	req := RobotLoginRequest{
		Repository:  repository,
		AccessToken: accessToken,
	}

	userAuth := &UserAuth{}
	err := callProdAPI(ctx, "nsl.signin.SigninService/RobotLogin", req, func(dec *json.Decoder) error {
		return dec.Decode(userAuth)
	})
	return userAuth, err
}

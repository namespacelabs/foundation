// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package fnapi

import (
	"context"
	"encoding/json"
)

type CheckRequest struct {
	UserData string `json:"userData"`
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
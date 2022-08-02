// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package fnapi

import (
	"context"
	"encoding/json"
)

type CreateDemoRequest struct {
	OpaqueUserAuth []byte `json:"opaque_user_auth,omitempty"`
	Name           string `json:"name,omitempty"`
	Private        bool   `json:"private,omitempty"`
}

type CreateDemoResponse struct {
	Url string `json:"url"`
}

func CreateDemo(ctx context.Context, ua *UserAuth, private bool, name string) (string, error) {
	req := CreateDemoRequest{
		OpaqueUserAuth: ua.Opaque,
		Private:        private,
		Name:           name,
	}

	resp := &CreateDemoResponse{}
	err := callProdAPI(ctx, "nsl.demo.DemoService/CreateDemo", req, func(dec *json.Decoder) error {
		return dec.Decode(resp)
	})

	return resp.Url, err
}

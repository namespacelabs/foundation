// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package fnapi

import (
	"context"
	"encoding/json"
	"fmt"
)

type CreateWorkspaceRequest struct {
	OpaqueUserAuth []byte `json:"opaque_user_auth,omitempty"`
	Name           string `json:"name,omitempty"`
	Private        bool   `json:"private,omitempty"`
}

type CreateWorkspaceResponse struct {
	Url                  string `json:"url,omitempty"`
	NeedsRepoPermissions bool   `json:"needs_repo_permissions,omitempty"`
}

func CreateWorkspace(ctx context.Context, ua *UserAuth, private bool, name string) (string, error) {
	req := CreateWorkspaceRequest{
		OpaqueUserAuth: ua.Opaque,
		Private:        private,
		Name:           name,
	}

	resp := &CreateWorkspaceResponse{}
	err := callProdAPI(ctx, "nsl.demo.WorkspaceService/CreateWorkspace", req, func(dec *json.Decoder) error {
		return dec.Decode(resp)
	})

	if resp.NeedsRepoPermissions {
		return "", fmt.Errorf("Don't have permissions to create repo.")
	}

	return resp.Url, err
}

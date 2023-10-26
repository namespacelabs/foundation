// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package auth

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"namespacelabs.dev/foundation/internal/clerk"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
)

const userAuthJson = "auth.json"

type UserAuth struct {
	Username       string `json:"username,omitempty"`
	Org            string `json:"org,omitempty"` // The organization this user is acting as. Only really relevant for robot accounts which authenticate against a repository.
	InternalOpaque []byte `json:"opaque,omitempty"`

	Clerk          *clerk.State `json:"clerk,omitempty"`
	IsGithubAction bool         `json:"is_github_action,omitempty"`
}

func StoreMarshalledUser(ctx context.Context, userAuthData []byte) error {
	configDir, err := dirs.Ensure(dirs.Config())
	if err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(configDir, userAuthJson), userAuthData, 0600); err != nil {
		return fnerrors.New("failed to write user auth data: %w", err)
	}

	return nil
}

func LoadUser() (*UserAuth, error) {
	dir, err := dirs.Config()
	if err != nil {
		return nil, err
	}

	p := filepath.Join(dir, userAuthJson)
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fnerrors.ReauthError("not logged in")
		}

		return nil, err
	}

	userAuth := &UserAuth{}
	if err := json.Unmarshal(data, userAuth); err != nil {
		return nil, err
	}

	return userAuth, nil
}

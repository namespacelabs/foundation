// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package auth

import (
	"encoding/json"
	"os"
	"path/filepath"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
)

const userAuthJson = "auth.json"

type UserAuth struct {
	Username string `json:"username,omitempty"`
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

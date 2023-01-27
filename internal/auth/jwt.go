// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package auth

import (
	"os"
	"path/filepath"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
)

const tokenTxt = "token.txt"

func StoreToken(token string) error {
	configDir, err := dirs.Ensure(dirs.Config())
	if err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(configDir, tokenTxt), []byte(token), 0600); err != nil {
		return fnerrors.New("failed to write token data: %w", err)
	}

	return nil
}

func LoadToken() (string, error) {
	dir, err := dirs.Config()
	if err != nil {
		return "", err
	}

	p := filepath.Join(dir, tokenTxt)
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			// TODO should we suggest Github token exchange, too?
			return "", ErrRelogin
		}

		return "", err
	}

	return string(data), nil
}

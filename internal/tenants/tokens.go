// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package tenants

import (
	"context"
	"os"
	"path/filepath"

	"namespacelabs.dev/foundation/internal/auth"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/github"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
)

const (
	tokenTxt          = "tenant_token.txt"
	githubJWTAudience = "nscloud.dev/inline-token"
)

func storeToken(token string) error {
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
			return "", auth.ErrRelogin
		}

		return "", err
	}

	return string(data), nil
}

func RefreshTokenForGithubAction(ctx context.Context) error {
	if os.Getenv("GITHUB_ACTIONS") != "true" {
		return fnerrors.New("not running in a GitHub action")
	}

	jwt, err := github.JWT(ctx, githubJWTAudience)
	if err != nil {
		return err
	}

	token, err := fnapi.ExchangeGithubToken(ctx, jwt)
	if err != nil {
		return err
	}

	return storeToken(token)
}

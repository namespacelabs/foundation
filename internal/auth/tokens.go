// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package auth

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
)

const (
	tokenTxt          = "token.json"
	GithubJWTAudience = "nscloud.dev/inline-token"
)

type Token struct {
	TenantToken string `json:"tenant_token,omitempty"`
}

func StoreTenantToken(token string) error {
	data, err := json.Marshal(Token{TenantToken: token})
	if err != nil {
		return err
	}

	configDir, err := dirs.Ensure(dirs.Config())
	if err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(configDir, tokenTxt), data, 0600); err != nil {
		return fnerrors.New("failed to write token data: %w", err)
	}

	return nil
}

func LoadTenantToken() (*Token, error) {
	dir, err := dirs.Config()
	if err != nil {
		return nil, err
	}

	p := filepath.Join(dir, tokenTxt)
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}

	token := &Token{}
	if err := json.Unmarshal(data, token); err != nil {
		return nil, err
	}

	claims := jwt.RegisteredClaims{}
	if _, _, err := new(jwt.Parser).ParseUnverified(strings.TrimPrefix("nsct_", token.TenantToken), claims); err != nil {
		return nil, err
	}

	// If stored token is expired, we remove it and return [fs.ErrNotExist].
	if !claims.VerifyExpiresAt(time.Now(), true) {
		if err := os.Remove(p); err != nil {
			return nil, err
		}
		return nil, fs.ErrNotExist
	}

	return token, nil
}

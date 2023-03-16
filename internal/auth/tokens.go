// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"namespacelabs.dev/foundation/internal/cli/fncobra/name"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
)

const (
	GithubJWTAudience = "nscloud.dev/inline-token"

	tokenTxt      = "token.json"
	adminTokenTxt = "admin_token.json"
)

type Token struct {
	TenantToken string `json:"tenant_token,omitempty"`
}

func (t *Token) Raw() string {
	return t.TenantToken
}

func storeToken(token, loc string) error {
	data, err := json.Marshal(Token{TenantToken: token})
	if err != nil {
		return err
	}

	configDir, err := dirs.Ensure(dirs.Config())
	if err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(configDir, loc), data, 0600); err != nil {
		return fnerrors.New("failed to write token data: %w", err)
	}

	return nil
}

func StoreAdminToken(token string) error {
	return storeToken(token, adminTokenTxt)
}

func StoreTenantToken(token string) error {
	return storeToken(token, tokenTxt)
}

func loadToken(ctx context.Context, loc string) (*Token, error) {
	dir, err := dirs.Config()
	if err != nil {
		return nil, err
	}

	p := filepath.Join(dir, loc)
	data, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, ErrRelogin
		}

		return nil, err
	}

	token := &Token{}
	if err := json.Unmarshal(data, token); err != nil {
		fmt.Fprintf(console.Debug(ctx), "failed to unmarshal cached tenant token: %v\n", err)
		return nil, ErrRelogin
	}

	claims := jwt.RegisteredClaims{}
	parser := jwt.Parser{}
	if _, _, err := parser.ParseUnverified(strings.TrimPrefix(token.TenantToken, "nsct_"), &claims); err != nil {
		fmt.Fprintf(console.Debug(ctx), "failed to parse tenant JWT: %v\n", err)
		return nil, ErrRelogin
	}

	if !claims.VerifyExpiresAt(time.Now(), true) {
		fmt.Fprintf(console.Debug(ctx), "tenant JWT has expired\n")
		return nil, ErrRelogin
	}

	return token, nil
}

func LoadAdminToken(ctx context.Context) (*Token, error) {
	tok, err := loadToken(ctx, adminTokenTxt)
	if err != nil {
		if err == ErrRelogin {
			return nil, fnerrors.New("not logged in, please run `%s login --fnapi_admin --workspace={tenant_to_impersonate}`", name.CmdName)
		}
		return nil, err
	}

	return tok, nil
}

func LoadTenantToken(ctx context.Context) (*Token, error) {
	return loadToken(ctx, tokenTxt)
}

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

	tokenJson      = "token.json"
	adminTokenJson = "admin_token.json"
)

type Token struct {
	BearerToken string `json:"bearer_token,omitempty"`
}

// TODO: remove when legacy token.json format is not used anymore.
func (t *Token) UnmarshalJSON(data []byte) error {
	var migrateToken struct {
		BearerToken string `json:"bearer_token,omitempty"`
		TenantToken string `json:"tenant_token,omitempty"`
	}

	if err := json.Unmarshal(data, &migrateToken); err != nil {
		return err
	}

	t.BearerToken = migrateToken.BearerToken
	if t.BearerToken == "" {
		t.BearerToken = migrateToken.TenantToken
	}

	return nil
}

func (t *Token) Raw() string {
	return t.BearerToken
}

func storeToken(token, loc string) error {
	data, err := json.Marshal(Token{BearerToken: token})
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
	return storeToken(token, adminTokenJson)
}

func StoreTenantToken(token string) error {
	return storeToken(token, tokenJson)
}

func loadUserToken(ctx context.Context, filename string, target time.Time) (*Token, error) {
	dir, err := dirs.Config()
	if err != nil {
		return nil, err
	}

	p := filepath.Join(dir, filename)
	token, err := LoadTokenFromPath(ctx, p, target)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fnerrors.ReloginError()
		}

		return nil, err
	}

	return token, nil
}

func LoadTokenFromPath(ctx context.Context, path string, validAt time.Time) (*Token, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	token := &Token{}
	if err := json.Unmarshal(data, token); err != nil {
		fmt.Fprintf(console.Debug(ctx), "failed to unmarshal cached tenant token: %v\n", err)
		return nil, fnerrors.ReloginError()
	}

	claims := jwt.RegisteredClaims{}
	parser := jwt.Parser{}
	var rawToken string
	switch {
	case strings.HasPrefix(token.BearerToken, "nsct_"):
		rawToken = strings.TrimPrefix(token.BearerToken, "nsct_")
	case strings.HasPrefix(token.BearerToken, "nscw_"):
		rawToken = strings.TrimPrefix(token.BearerToken, "nscw_")
	default:
		fmt.Fprintf(console.Debug(ctx), "unknown token format\n")
		return nil, fnerrors.ReloginError()
	}

	if _, _, err := parser.ParseUnverified(rawToken, &claims); err != nil {
		fmt.Fprintf(console.Debug(ctx), "failed to parse tenant JWT: %v\n", err)
		return nil, fnerrors.ReloginError()
	}

	if !claims.VerifyExpiresAt(validAt, true) {
		fmt.Fprintf(console.Debug(ctx), "tenant JWT has expired\n")
		return nil, fnerrors.ReloginError()
	}

	return token, nil
}

func LoadAdminToken(ctx context.Context) (*Token, error) {
	tok, err := loadUserToken(ctx, adminTokenJson, time.Now())
	if err != nil {
		var relogin *fnerrors.ReloginErr
		if errors.As(err, &relogin) {
			return nil, fnerrors.New("not logged in, please run `%s login --fnapi_admin --workspace={tenant_to_impersonate}`", name.CmdName)
		}
		return nil, err
	}

	return tok, nil
}

func LoadTenantToken(ctx context.Context) (*Token, error) {
	return loadUserToken(ctx, tokenJson, time.Now())
}

func EnsureTokenValidAt(ctx context.Context, target time.Time) error {
	_, err := loadUserToken(ctx, tokenJson, target)
	return err
}

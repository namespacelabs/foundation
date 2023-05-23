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
	"github.com/spf13/pflag"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/metadata"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
)

const (
	GithubJWTAudience = "nscloud.dev/inline-token"

	defaultTokenLoc = "token.json"
)

var Workspace string

func SetupFlags(flags *pflag.FlagSet) {
	flags.StringVar(&Workspace, "workspace", "", "Select a workspace log in to.")

	_ = flags.MarkHidden("workspace")
}

func tokenLoc() string {
	if Workspace == "" {
		return defaultTokenLoc
	}

	return fmt.Sprintf("token_%s.json", Workspace)
}

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

func StoreTenantToken(token string) error {
	data, err := json.Marshal(Token{BearerToken: token})
	if err != nil {
		return err
	}

	configDir, err := dirs.Ensure(dirs.Config())
	if err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(configDir, tokenLoc()), data, 0600); err != nil {
		return fnerrors.New("failed to write token data: %w", err)
	}

	return nil
}

func loadWorkspaceToken(ctx context.Context, target time.Time) (*Token, error) {
	dir, err := dirs.Config()
	if err != nil {
		return nil, err
	}

	p := filepath.Join(dir, tokenLoc())
	token, err := LoadTokenFromPath(ctx, p, target)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fnerrors.ReauthError("not logged in")
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

	return LoadToken(ctx, data, validAt)
}

func LoadToken(ctx context.Context, data []byte, validAt time.Time) (*Token, error) {
	token := &Token{}
	if err := json.Unmarshal(data, token); err != nil {
		fmt.Fprintf(console.Debug(ctx), "failed to unmarshal cached tenant token: %v\n", err)
		return nil, fnerrors.ReauthError("not logged in")
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
		return nil, fnerrors.ReauthError("not logged in")
	}

	if _, _, err := parser.ParseUnverified(rawToken, &claims); err != nil {
		fmt.Fprintf(console.Debug(ctx), "failed to parse tenant JWT: %v\n", err)
		return nil, fnerrors.ReauthError("not logged in")
	}

	if !claims.VerifyExpiresAt(validAt, true) {
		if strings.HasPrefix(token.BearerToken, "nscw_") {
			return nil, fnerrors.InternalError("workload token expired")
		}

		return nil, fnerrors.ReauthError("login token expired")
	}

	return token, nil
}

func LoadTenantToken(ctx context.Context) (*Token, error) {
	return loadWorkspaceToken(ctx, time.Now())
}

func EnsureTokenValidAt(ctx context.Context, target time.Time) error {
	_, err := loadWorkspaceToken(ctx, target)
	return err
}

func FetchTokenFromSpec(ctx context.Context, spec string) (*Token, error) {
	t, err := metadata.FetchTokenFromSpec(ctx, spec)
	if err != nil {
		return nil, err
	}

	return &Token{BearerToken: t}, nil
}

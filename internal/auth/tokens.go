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
	"sort"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
)

const (
	tokenTxt          = "token.json"
	GithubJWTAudience = "nscloud.dev/inline-token"
)

var ErrTokenNotExist = errors.New("token does not exist")

type cachedTokens map[string]Token

type Token struct {
	Scopes      []string `json:"scopes,omitempty"`
	TenantToken string   `json:"tenant_token,omitempty"`
}

func StoreTenantToken(ctx context.Context, username string, token Token) error {
	dir, err := dirs.Config()
	if err != nil {
		return err
	}

	f := filepath.Join(dir, tokenTxt)
	cache, err := getCachedTokens(ctx, f)
	if err != nil {
		return err
	}

	sort.Strings(token.Scopes)
	cache[username+strings.Join(token.Scopes, "")] = token

	data, err := json.Marshal(cache)
	if err != nil {
		return err
	}

	if err := os.WriteFile(f, data, 0600); err != nil {
		return fnerrors.New("failed to write token cache: %w", err)
	}

	return nil
}

func LoadTenantToken(ctx context.Context, username string, scopes []string) (*Token, error) {
	dir, err := dirs.Config()
	if err != nil {
		return nil, err
	}

	f := filepath.Join(dir, tokenTxt)
	cache, err := getCachedTokens(ctx, f)
	if err != nil {
		return nil, err
	}

	sort.Strings(scopes)
	token, ok := cache[username+strings.Join(scopes, "")]
	if !ok {
		return nil, ErrTokenNotExist
	}

	claims := jwt.RegisteredClaims{}
	parser := jwt.Parser{}
	if _, _, err := parser.ParseUnverified(strings.TrimPrefix(token.TenantToken, "nsct_"), &claims); err != nil {
		fmt.Fprintf(console.Debug(ctx), "failed to parse tenant JWT: %v\n", err)
		if err := os.Remove(f); err != nil {
			return nil, err
		}
		return nil, ErrTokenNotExist
	}

	if !claims.VerifyExpiresAt(time.Now(), true) {
		fmt.Fprintf(console.Debug(ctx), "tenant JWT has expired\n")
		if err := os.Remove(f); err != nil {
			return nil, err
		}
		return nil, ErrTokenNotExist
	}

	return &token, nil
}

func getCachedTokens(ctx context.Context, f string) (cachedTokens, error) {
	tokens := cachedTokens{}

	data, err := os.ReadFile(f)
	switch {
	case err == nil:
		if err := json.Unmarshal(data, &tokens); err != nil {
			fmt.Fprintf(console.Debug(ctx), "failed to unmarshal cached tenant tokens: %v\n", err)
			if err := os.Remove(f); err != nil {
				return nil, err
			}
		}

		return tokens, nil
	case errors.Is(err, fs.ErrNotExist):
		return tokens, nil
	default:
		return nil, err
	}
}

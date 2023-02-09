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
	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
)

const (
	tokenTxt          = "token.json"
	GithubJWTAudience = "nscloud.dev/inline-token"
)

var ErrTokenNotExist = errors.New("token does not exist")

type Token struct {
	Username    string   `json:"username,omitempty"`
	Scopes      []string `json:"scopes,omitempty"`
	TenantToken string   `json:"tenant_token,omitempty"`
}

func (t Token) Equal(other Token) bool {
	if t.Username == other.Username {
		slices.Sort(t.Scopes)
		slices.Sort(other.Scopes)
		return slices.Equal(t.Scopes, other.Scopes)
	}

	return false
}

func StoreTenantToken(ctx context.Context, token Token) error {
	dir, err := dirs.Config()
	if err != nil {
		return err
	}

	f := filepath.Join(dir, tokenTxt)
	cache, err := getCachedTokens(ctx, f)
	if err != nil {
		return err
	}

	cache.setToken(token)
	return cache.store(f)
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

	token, ok := cache.getToken(username, scopes)
	if !ok {
		return nil, ErrTokenNotExist
	}

	claims := jwt.RegisteredClaims{}
	parser := jwt.Parser{}
	if _, _, err := parser.ParseUnverified(strings.TrimPrefix(token.TenantToken, "nsct_"), &claims); err != nil {
		fmt.Fprintf(console.Debug(ctx), "failed to parse tenant JWT: %v\n", err)
		cache.deleteToken(token)
		if err := cache.store(f); err != nil {
			return nil, err
		}
		return nil, ErrTokenNotExist
	}

	if isExpired(claims) {
		fmt.Fprintf(console.Debug(ctx), "tenant JWT has expired\n")
		cache.deleteToken(token)
		if err := cache.store(f); err != nil {
			return nil, err
		}
		return nil, ErrTokenNotExist
	}

	return &token, nil
}

func isExpired(claims jwt.RegisteredClaims) bool {
	return !claims.VerifyExpiresAt(time.Now(), true)
}

type cachedTokens struct {
	tokens []Token
}

func (c *cachedTokens) setToken(token Token) {
	idx := slices.IndexFunc(c.tokens, token.Equal)
	if idx != -1 {
		c.tokens[idx] = token
		return
	}

	c.tokens = append(c.tokens, token)
}

func (c *cachedTokens) getToken(username string, scopes []string) (Token, bool) {
	idx := slices.IndexFunc(c.tokens, Token{Username: username, Scopes: scopes}.Equal)
	if idx != -1 {
		return c.tokens[idx], true
	}
	return Token{}, false
}

func (c *cachedTokens) deleteToken(token Token) {
	idx := slices.IndexFunc(c.tokens, token.Equal)
	if idx == -1 {
		return
	}

	temp := c.tokens[:idx]
	if idx < len(c.tokens)-1 {
		temp = append(temp, c.tokens[idx+1:]...)
	}
	c.tokens = temp
}

func (c *cachedTokens) store(f string) error {
	data, err := json.Marshal(c.tokens)
	if err != nil {
		return err
	}

	if err := os.WriteFile(f, data, 0600); err != nil {
		return fnerrors.New("failed to write token cache: %w", err)
	}
	return nil
}

func getCachedTokens(ctx context.Context, f string) (*cachedTokens, error) {
	data, err := os.ReadFile(f)
	switch {
	case err == nil:
		tokens := &cachedTokens{}
		if err := json.Unmarshal(data, &tokens.tokens); err != nil {
			fmt.Fprintf(console.Debug(ctx), "failed to unmarshal cached tenant tokens: %v\n", err)
			if err := os.Remove(f); err != nil {
				return nil, err
			}
		}

		return tokens, nil
	case errors.Is(err, fs.ErrNotExist):
		return &cachedTokens{}, nil
	default:
		return nil, err
	}
}

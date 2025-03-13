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
	path string

	BearerToken  string `json:"bearer_token,omitempty"`
	SessionToken string `json:"session_token,omitempty"`
}

// TODO: remove when legacy token.json format is not used anymore.
func (t *Token) UnmarshalJSON(data []byte) error {
	var migrateToken struct {
		BearerToken  string `json:"bearer_token,omitempty"`
		SessionToken string `json:"session_token,omitempty"`
		TenantToken  string `json:"tenant_token,omitempty"`
	}

	if err := json.Unmarshal(data, &migrateToken); err != nil {
		return err
	}

	t.BearerToken = migrateToken.BearerToken
	t.SessionToken = migrateToken.SessionToken
	if migrateToken.TenantToken != "" {
		t.BearerToken = migrateToken.TenantToken
	}

	return nil
}

type TokenClaims struct {
	jwt.RegisteredClaims

	TenantID      string `json:"tenant_id"`
	InstanceID    string `json:"instance_id"`
	OwnerID       string `json:"owner_id"`
	PrimaryRegion string `json:"primary_region"`
}

func (t *Token) IsSessionToken() bool { return t.SessionToken != "" }

func (t *Token) Claims(ctx context.Context) (*TokenClaims, error) {
	if t.SessionToken != "" {
		return parseClaims(ctx, strings.TrimPrefix(t.SessionToken, "st_"))
	}

	switch {
	case strings.HasPrefix(t.BearerToken, "nsct_"):
		return parseClaims(ctx, strings.TrimPrefix(t.BearerToken, "nsct_"))
	case strings.HasPrefix(t.BearerToken, "nscw_"):
		return parseClaims(ctx, strings.TrimPrefix(t.BearerToken, "nscw_"))
	default:
		return nil, fnerrors.ReauthError("not logged in")
	}
}

func (t *Token) PrimaryRegion(ctx context.Context) (string, error) {
	claims, err := t.Claims(ctx)
	if err != nil {
		return "", err
	}

	return claims.PrimaryRegion, nil
}

func parseClaims(ctx context.Context, raw string) (*TokenClaims, error) {
	parser := jwt.Parser{}

	var claims TokenClaims
	if _, _, err := parser.ParseUnverified(raw, &claims); err != nil {
		fmt.Fprintf(console.Debug(ctx), "parsing claims %q failed: %v\n", raw, err)
		return nil, fnerrors.ReauthError("not logged in")
	}

	return &claims, nil
}

func (t *Token) IssueToken(ctx context.Context, minDur time.Duration, issueShortTerm func(context.Context, string, time.Duration) (string, error), skipCache bool) (string, error) {
	if t.SessionToken != "" {
		if skipCache {
			return issueShortTerm(ctx, t.SessionToken, minDur)
		}

		if t.path != "" {
			cachePath := filepath.Join(filepath.Dir(t.path), "token.cache")
			cacheContents, err := os.ReadFile(cachePath)
			if err != nil {
				if !os.IsNotExist(err) {
					return "", err
				}
			} else {
				cacheClaims, err := parseClaims(ctx, strings.TrimPrefix(string(cacheContents), "nsct_"))
				if err != nil {
					return "", err
				}

				sessionClaims, err := t.Claims(ctx)
				if err != nil {
					return "", err
				}

				if cacheClaims.TenantID == sessionClaims.TenantID {
					if cacheClaims.VerifyExpiresAt(time.Now().Add(minDur), true) {
						return string(cacheContents), nil
					}
				}
			}
		}

		dur := 2 * minDur
		if dur > 8*time.Hour {
			dur = 8 * time.Hour
		}

		newToken, err := issueShortTerm(ctx, t.SessionToken, dur)
		if err == nil && t.path != "" {
			cachePath := filepath.Join(filepath.Dir(t.path), "token.cache")
			if err := os.WriteFile(cachePath, []byte(newToken), 0600); err != nil {
				fmt.Fprintf(console.Warnings(ctx), "Failed to write token cache: %v\n", err)
			}
		}

		return newToken, err
	}

	return t.BearerToken, nil
}

type IssueCertFunc func(context.Context, string, string) (string, error)

func (t *Token) ExchangeForSessionClientCert(ctx context.Context, publicKeyPem string, issueFromSession IssueCertFunc) (string, error) {
	if t.SessionToken == "" {
		return "", fnerrors.Newf("ExchangeForSessionClientCert called on a token which is not a session token")
	}

	return issueFromSession(ctx, t.SessionToken, publicKeyPem)
}

func StoreTenantToken(token string) error {
	return StoreToken(Token{BearerToken: token})
}

func StoreToken(token Token) error {
	data, err := json.Marshal(token)
	if err != nil {
		return err
	}

	configDir, err := dirs.Ensure(dirs.Config())
	if err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(configDir, tokenLoc()), data, 0600); err != nil {
		return fnerrors.Newf("failed to write token data: %w", err)
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

	token := &Token{path: path}
	if err := json.Unmarshal(data, token); err != nil {
		fmt.Fprintf(console.Debug(ctx), "failed to unmarshal cached tenant token: %v\n", err)
		return nil, fnerrors.ReauthError("not logged in")
	}

	claims, err := token.Claims(ctx)
	if err != nil {
		return nil, err
	}

	if !claims.VerifyExpiresAt(validAt, true) {
		if token.SessionToken != "" {
			return nil, fnerrors.ReauthError("session expired")
		}

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

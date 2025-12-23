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
	"regexp"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/metadata"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
	"namespacelabs.dev/integrations/auth"
)

const (
	GithubJWTAudience = "nscloud.dev/inline-token"

	defaultTokenLoc = "token.json"
)

var Workspace string
var Keychain string

func SetupFlags(flags *pflag.FlagSet) {
	flags.StringVar(&Workspace, "workspace", "", "Select a workspace log in to.")
	flags.StringVar(&Keychain, "keychain", "", "Use the specified token keychain.")

	_ = flags.MarkHidden("workspace")
}

func tokenLoc() string {
	if Workspace == "" {
		return defaultTokenLoc
	}

	return fmt.Sprintf("token_%s.json", Workspace)
}

type IssueShortTermFunc func(context.Context, *Token, time.Duration) (string, error)

type Token struct {
	path string

	ReIssue IssueShortTermFunc

	StoredToken

	revokable bool // Set for revokable tokens that are validated server-side
}

func (t *Token) IsSessionToken() bool { return t.SessionToken != "" }

func (t *Token) ExpiresAt(ctx context.Context) (time.Time, bool, error) {
	if t.revokable {
		return time.Time{}, false, nil
	}

	claims, err := t.Claims(ctx)
	if err != nil {
		return time.Time{}, false, err
	}

	return claims.ExpiresAt.Time, true, nil
}

func (t *Token) Claims(ctx context.Context) (*auth.TokenClaims, error) {
	if t.SessionToken != "" {
		return extractClaims(t.SessionToken)
	}

	return extractClaims(t.TenantToken)
}

func (t *Token) PreferredRegion(ctx context.Context) (string, error) {
	if t.revokable {
		return "", nil
	}

	claims, err := t.Claims(ctx)
	if err != nil {
		return "", err
	}

	if claims.WorkloadRegion != "" {
		return claims.WorkloadRegion, nil
	}

	if claims.PrimaryRegion != "" {
		return claims.PrimaryRegion, nil
	}

	return "", nil
}

func extractClaims(token string) (*auth.TokenClaims, error) {
	claims, err := auth.ExtractClaims(token)
	if err != nil {
		if errors.Is(err, auth.ErrNotLoggedIn) {
			return nil, fnerrors.ReauthError("not logged in")
		}

		return nil, err
	}

	return claims, nil
}

func (t *Token) IssueToken(ctx context.Context, minDur time.Duration, skipCache bool) (string, error) {
	// Revokable tokens are validated server-side, just return them
	if t.revokable {
		fmt.Fprintf(console.Debug(ctx), "Re-using revokable token (server-side validation)...\n")
		return t.TenantToken, nil
	}

	if t.TenantToken != "" && !skipCache {
		claims, err := extractClaims(t.TenantToken)
		if err != nil {
			return "", err
		}

		if claims != nil {
			if claims.VerifyExpiresAt(time.Now().Add(minDur), true) {
				fmt.Fprintf(console.Debug(ctx), "Existing tenant token meets minimum duration %v, re-using...\n", minDur)

				return t.TenantToken, nil
			}
		}
	}

	if t.ReIssue == nil {
		return "", fnerrors.ReauthError("tenant token is expired, and can't re-issue a new one")
	}

	if skipCache {
		return t.ReIssue(ctx, t, minDur)
	}

	if t.path != "" {
		cachePath := filepath.Join(filepath.Dir(t.path), "token.cache")
		cacheContents, err := os.ReadFile(cachePath)
		if err != nil {
			if !os.IsNotExist(err) {
				return "", err
			}
		} else {
			claims, err := extractClaims(string(cacheContents))
			if err != nil {
				return "", err
			}

			if claims != nil {
				sessionClaims, err := t.Claims(ctx)
				if err != nil {
					return "", err
				}

				if claims.TenantID == sessionClaims.TenantID {
					fmt.Fprintf(console.Debug(ctx), "Re-loaded tenant token from cache (expires at %v).\n", claims.ExpiresAt.Time)

					if claims.VerifyExpiresAt(time.Now().Add(minDur), true) {
						return string(cacheContents), nil
					}
				}
			}
		}
	}

	dur := min(2*minDur, 8*time.Hour)

	newToken, err := t.ReIssue(ctx, t, dur)
	if err == nil && t.path != "" {
		cachePath := filepath.Join(filepath.Dir(t.path), "token.cache")
		if err := os.WriteFile(cachePath, []byte(newToken), 0600); err != nil {
			fmt.Fprintf(console.Warnings(ctx), "Failed to write token cache: %v\n", err)
		}
	}

	return newToken, err
}

type IssueCertFunc func(context.Context, string, string) (string, error)

func (t *Token) ExchangeForSessionClientCert(ctx context.Context, publicKeyPem string, issueFromSession IssueCertFunc) (string, error) {
	if t.SessionToken == "" {
		return "", fnerrors.Newf("ExchangeForSessionClientCert called on a token which is not a session token")
	}

	return issueFromSession(ctx, t.SessionToken, publicKeyPem)
}

func StoreTenantToken(token string) error {
	return StoreToken(StoredToken{TenantToken: token})
}

type StoredToken struct {
	TenantToken  string `json:"bearer_token,omitempty"`
	SessionToken string `json:"session_token,omitempty"`
}

// TODO: remove when legacy token.json format is not used anymore.
func (t *StoredToken) UnmarshalJSON(data []byte) error {
	var migrateToken struct {
		BearerToken  string `json:"bearer_token,omitempty"`
		SessionToken string `json:"session_token,omitempty"`
		TenantToken  string `json:"tenant_token,omitempty"`
	}

	if err := json.Unmarshal(data, &migrateToken); err != nil {
		return err
	}

	t.TenantToken = migrateToken.BearerToken
	t.SessionToken = migrateToken.SessionToken
	if migrateToken.TenantToken != "" {
		t.TenantToken = migrateToken.TenantToken
	}

	return nil
}

func StoreToken(token StoredToken) error {
	data, err := json.Marshal(token)
	if err != nil {
		return err
	}

	configDir, err := dirs.Ensure(dirs.Config())
	if err != nil {
		return err
	}

	p := filepath.Join(configDir, tokenLoc())
	if Keychain != "" {
		p, err = keychainPath(configDir)
		if err != nil {
			return err
		}

		if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			return err
		}
	}

	if err := os.WriteFile(p, data, 0600); err != nil {
		return fnerrors.Newf("failed to write token data: %w", err)
	}

	return nil
}

func DeleteStoredToken() error {
	dir, err := dirs.Config()
	if err != nil {
		return err
	}

	conf := filepath.Join(dir, tokenLoc())
	if _, err := os.Stat(conf); err == nil {
		return os.Remove(conf)
	}

	return nil
}

func HasKeychain() bool {
	return Keychain != ""
}

var validKeychainName = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

func keychainPath(configDir string) (string, error) {
	if len(Keychain) == 0 || len(Keychain) > 63 || !validKeychainName.MatchString(Keychain) {
		return "", fnerrors.New("invalid keychain name: must be 1-63 lowercase alphanumeric characters or hyphens, and cannot start or end with a hyphen")
	}

	return filepath.Join(configDir, "keychain", Keychain+".json"), nil
}

func loadWorkspaceToken(ctx context.Context, issue IssueShortTermFunc, target time.Time) (*Token, error) {
	dir, err := dirs.Config()
	if err != nil {
		return nil, err
	}

	p := filepath.Join(dir, tokenLoc())
	if Keychain != "" {
		p, err = keychainPath(dir)
		if err != nil {
			return nil, err
		}
	}

	token, err := LoadTokenFromPath(ctx, issue, p, target)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fnerrors.ReauthError("not logged in")
		}

		return nil, err
	}

	return token, nil
}

func LoadTokenFromPath(ctx context.Context, issue IssueShortTermFunc, path string, validAt time.Time) (*Token, error) {
	fmt.Fprintf(console.Debug(ctx), "Loading credentials from %q...\n", path)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	token := &Token{path: path, ReIssue: issue}
	if err := json.Unmarshal(data, &token.StoredToken); err != nil {
		fmt.Fprintf(console.Debug(ctx), "failed to unmarshal cached tenant token: %v\n", err)
		return nil, fnerrors.ReauthError("not logged in")
	}

	// Revokable tokens (nsrt_ prefix) are validated server-side, skip local checks
	if strings.HasPrefix(token.TenantToken, "nsrt_") {
		fmt.Fprintf(console.Debug(ctx), "Using revokable token (server-side validation)\n")
		token.revokable = true
		return token, nil
	}

	claims, err := token.Claims(ctx)
	if err != nil {
		return nil, err
	}

	if !claims.VerifyExpiresAt(validAt, true) {
		if token.SessionToken != "" {
			return nil, fnerrors.ReauthError("session expired")
		}

		if strings.HasPrefix(token.TenantToken, "nscw_") {
			return nil, fnerrors.InternalError("workload token expired")
		}

		return nil, fnerrors.ReauthError("login token expired")
	}

	fmt.Fprintf(console.Debug(ctx), "Credentials valid until %v.\n", claims.ExpiresAt.Time)

	return token, nil
}

func LoadTenantToken(ctx context.Context, issue IssueShortTermFunc) (*Token, error) {
	return loadWorkspaceToken(ctx, issue, time.Now())
}

func EnsureTokenValidAt(ctx context.Context, issue IssueShortTermFunc, target time.Time) error {
	_, err := loadWorkspaceToken(ctx, issue, target)
	return err
}

func FetchTokenFromSpec(ctx context.Context, issue IssueShortTermFunc, spec string) (*Token, error) {
	t, err := metadata.FetchTokenFromSpec(ctx, spec)
	if err != nil {
		return nil, err
	}

	return &Token{StoredToken: StoredToken{TenantToken: t}, ReIssue: issue}, nil
}

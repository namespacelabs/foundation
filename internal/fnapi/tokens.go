// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fnapi

import (
	"context"
	"os"
	"time"

	"namespacelabs.dev/foundation/internal/auth"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/std/tasks"
)

type Token interface {
	IsSessionToken() bool
	Claims(context.Context) (*auth.TokenClaims, error)
	PrimaryRegion(context.Context) (string, error)
	IssueToken(context.Context, time.Duration, func(context.Context, string, time.Duration) (string, error)) (string, error)
}

func BearerToken(ctx context.Context, t Token) (string, error) {
	raw, err := t.IssueToken(ctx, 5*time.Minute, IssueTenantTokenFromSession)
	if err != nil {
		return "", err
	}

	return raw, nil
}

type ResolvedToken struct {
	BearerToken string

	PrimaryRegion string // Only available in tenant tokens.
}

func FetchToken(ctx context.Context) (Token, error) {
	return tasks.Return(ctx, tasks.Action("nsc.fetch-token").LogLevel(1), func(ctx context.Context) (*auth.Token, error) {
		spec, err := ResolveSpec()
		if err != nil {
			return nil, err
		}

		if spec != "" {
			return auth.FetchTokenFromSpec(ctx, spec)
		}

		if specified := os.Getenv("NSC_TOKEN_FILE"); specified != "" {
			return auth.LoadTokenFromPath(ctx, specified, time.Now())
		}

		return auth.LoadTenantToken(ctx)
	})
}

func IssueBearerToken(ctx context.Context) (ResolvedToken, error) {
	tok, err := FetchToken(ctx)
	if err != nil {
		return ResolvedToken{}, err
	}

	return IssueBearerTokenFromToken(ctx, tok)
}

func IssueBearerTokenFromToken(ctx context.Context, tok Token) (ResolvedToken, error) {
	primaryRegion, err := tok.PrimaryRegion(ctx)
	if err != nil {
		return ResolvedToken{}, err
	}

	bt, err := BearerToken(ctx, tok)
	if err != nil {
		return ResolvedToken{}, err
	}

	return ResolvedToken{BearerToken: bt, PrimaryRegion: primaryRegion}, nil
}

func IssueToken(ctx context.Context, minDur time.Duration) (string, error) {
	t, err := FetchToken(ctx)
	if err != nil {
		return "", err
	}

	return t.IssueToken(ctx, minDur, IssueTenantTokenFromSession)
}

func ResolveSpec() (string, error) {
	if spec := os.Getenv("NSC_TOKEN_SPEC"); spec != "" {
		return spec, nil
	}

	if specFile := os.Getenv("NSC_TOKEN_SPEC_FILE"); specFile != "" {
		contents, err := os.ReadFile(specFile)
		if err != nil {
			return "", fnerrors.New("failed to load spec: %w", err)
		}

		return string(contents), nil
	}

	return "", nil
}

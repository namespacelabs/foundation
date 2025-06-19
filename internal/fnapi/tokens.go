// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fnapi

import (
	"context"
	"errors"
	"os"
	"time"

	"google.golang.org/protobuf/types/known/emptypb"
	localauth "namespacelabs.dev/foundation/internal/auth"
	"namespacelabs.dev/foundation/internal/files"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/integrations/api/iam"
	"namespacelabs.dev/integrations/auth"
	"namespacelabs.dev/integrations/auth/aws"
)

type Token interface {
	IsSessionToken() bool
	Claims(context.Context) (*localauth.TokenClaims, error)
	PreferredRegion(context.Context) (string, error)
	IssueToken(context.Context, time.Duration, bool) (string, error)

	// This fails if it is not a session token.
	ExchangeForSessionClientCert(ctx context.Context, publicKeyPem string, issueFromSession localauth.IssueCertFunc) (string, error)
}

type ResolvedToken struct {
	BearerToken string

	PreferredRegion string
}

type ImpersonationSpec struct {
	PartnerId       string `json:"partner_id"`
	AWSIdentityPool string `json:"aws_identity_pool"`
}

func FetchToken(ctx context.Context) (*localauth.Token, error) {
	return tasks.Return(ctx, tasks.Action("nsc.fetch-token").LogLevel(1), func(ctx context.Context) (*localauth.Token, error) {
		if impersonate := os.Getenv("NSC_IMPERSONATE_TENANT_ID"); impersonate != "" {
			loc := os.Getenv("NSC_IMPERSONATION_SPEC_FILE")
			if loc == "" {
				return nil, fnerrors.BadInputError("NSC_IMPERSONATE_TENANT_ID requires NSC_IMPERSONATION_SPEC_FILE to be set")
			}

			var spec ImpersonationSpec
			if err := files.ReadJson(loc, &spec); err != nil {
				return nil, err
			}

			return ImpersonateFromSpec(ctx, spec, impersonate)
		}

		spec, err := ResolveSpec()
		if err != nil {
			return nil, err
		}

		if spec != "" {
			return localauth.FetchTokenFromSpec(ctx, IssueTenantTokenFromSession, spec)
		}

		if specified := os.Getenv("NSC_TOKEN_FILE"); specified != "" {
			return localauth.LoadTokenFromPath(ctx, IssueTenantTokenFromSession, specified, time.Now())
		}

		return localauth.LoadTenantToken(ctx, IssueTenantTokenFromSession)
	})
}

func ImpersonateFromSpec(ctx context.Context, spec ImpersonationSpec, tenantId string) (*localauth.Token, error) {
	if spec.PartnerId == "" {
		return nil, fnerrors.BadInputError("partner_id is required to be defined in the impersonation spec")
	}

	if spec.AWSIdentityPool == "" {
		return nil, fnerrors.BadInputError("aws_identity_pool is required to be defined in the impersonation spec")
	}

	fed, err := aws.Federation(ctx, spec.AWSIdentityPool, spec.PartnerId)
	if err != nil {
		return nil, err
	}

	iam, err := iam.NewClient(ctx, fed)
	if err != nil {
		return nil, err
	}

	src := auth.TenantTokenSource(iam, tenantId)

	token, err := src.IssueToken(ctx, 15*time.Minute, false)
	if err != nil {
		return nil, err
	}

	return &localauth.Token{
		ReIssue: func(ctx context.Context, _ *localauth.Token, dur time.Duration) (string, error) {
			return src.IssueToken(ctx, dur, false)
		},
		StoredToken: localauth.StoredToken{TenantToken: token},
	}, nil
}

func IssueBearerToken(ctx context.Context) (ResolvedToken, error) {
	tok, err := FetchToken(ctx)
	if err != nil {
		return ResolvedToken{}, err
	}

	return IssueBearerTokenFromToken(ctx, tok)
}

func IssueBearerTokenFromToken(ctx context.Context, tok Token) (ResolvedToken, error) {
	reg, err := tok.PreferredRegion(ctx)
	if err != nil {
		return ResolvedToken{}, err
	}

	bt, err := tok.IssueToken(ctx, 15*time.Minute, false)
	if err != nil {
		return ResolvedToken{}, err
	}

	return ResolvedToken{BearerToken: bt, PreferredRegion: reg}, nil
}

func IssueToken(ctx context.Context, minDur time.Duration) (string, error) {
	t, err := FetchToken(ctx)
	if err != nil {
		return "", err
	}

	return t.IssueToken(ctx, minDur, false)
}

func ResolveSpec() (string, error) {
	if spec := os.Getenv("NSC_TOKEN_SPEC"); spec != "" {
		return spec, nil
	}

	if specFile := os.Getenv("NSC_TOKEN_SPEC_FILE"); specFile != "" {
		contents, err := os.ReadFile(specFile)
		if err != nil {
			return "", fnerrors.Newf("failed to load spec: %w", err)
		}

		return string(contents), nil
	}

	return "", nil
}

func VerifySession(ctx context.Context, t Token) error {
	st := ""
	if t, ok := t.(*localauth.Token); ok && t.SessionToken != "" {
		st = t.SessionToken
	} else {
		return errors.New("not a session token")
	}

	return (Call[*emptypb.Empty]{
		Method: "nsl.signin.SigninService/VerifySession",
		IssueBearerToken: func(ctx context.Context) (ResolvedToken, error) {
			return ResolvedToken{BearerToken: st}, nil
		},
	}).Do(ctx, &emptypb.Empty{}, ResolveIAMEndpoint, nil)
}

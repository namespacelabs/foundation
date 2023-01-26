// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package auth

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"

	"namespacelabs.dev/foundation/internal/clerk"
	"namespacelabs.dev/foundation/internal/github"
)

const githubJWTAudience = "nscloud.dev/inline-token"

type authConfig struct {
	userAuth            *UserAuth
	githubOIDC          bool
	githubTokenExchange func(context.Context, string) (string, error)
}

type AuthOpt func(*authConfig)

// WithUserAuth an option to use existing `UserAuth` credentials to get JWT.
func WithUserAuth(userAuth *UserAuth) AuthOpt {
	return func(c *authConfig) {
		c.userAuth = userAuth
	}
}

// WithGithubOIDC an option to use Github OIDC provider to generate JWT.
func WithGithubOIDC(useGithubOIDC bool) AuthOpt {
	return func(c *authConfig) {
		c.githubOIDC = useGithubOIDC
	}
}

// WithGithubTokenExchange an option to exchange GitHub JWTs for Namespace JWTs.
func WithGithubTokenExchange(githubTokenExchange func(context.Context, string) (string, error)) AuthOpt {
	return func(c *authConfig) {
		c.githubTokenExchange = githubTokenExchange
	}
}

// GenerateToken generates token based on provided options. If no options provided an error would be returned.
func GenerateToken(ctx context.Context, opts ...AuthOpt) (string, error) {
	cfg := &authConfig{}
	for _, o := range opts {
		o(cfg)
	}
	switch {
	case cfg.userAuth != nil:
		if cfg.userAuth.Clerk != nil {
			jwt, err := clerk.JWT(ctx, cfg.userAuth.Clerk)
			if err != nil {
				if errors.Is(err, clerk.ErrUnauthorized) {
					return "", ErrRelogin
				}
				return "", err
			}
			return fmt.Sprintf("jwt:%s", jwt), nil
		}
		return base64.RawStdEncoding.EncodeToString(cfg.userAuth.InternalOpaque), nil

	case cfg.githubOIDC:
		if cfg.githubTokenExchange != nil {
			token, err := fetchTokenForGithub(ctx, cfg.githubTokenExchange)
			if err != nil {
				return "", err
			}

			return fmt.Sprintf("nsct_%s", token), nil
		}

		// TODO: remove this path
		jwt, err := github.JWT(ctx, githubJWTAudience)
		if err != nil {
			return "", err
		}

		return fmt.Sprintf("gh-jwt:%s", jwt), nil

	default:
		return "", ErrRelogin
	}
}

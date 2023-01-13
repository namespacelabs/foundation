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

type authConfig struct {
	userAuth   *UserAuth
	githubOIDC bool
}

type AuthOpt func(*authConfig)

func WithUserAuth(userAuth *UserAuth) AuthOpt {
	return func(c *authConfig) {
		c.userAuth = userAuth
	}
}

func WithGithubOIDC(useGithubOIDC bool) AuthOpt {
	return func(c *authConfig) {
		c.githubOIDC = useGithubOIDC
	}
}

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
		jwt, err := github.JWT(ctx, "") // XXX: do not set "audience" and use default one
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("gh-jwt:%s", jwt), nil

	default:
		return "", ErrRelogin
	}
}

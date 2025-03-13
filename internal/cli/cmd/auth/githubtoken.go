// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package auth

import (
	"context"
	"errors"
	"time"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/auth"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/github"
	ghenv "namespacelabs.dev/foundation/internal/github/env"
)

func NewExchangeGithubTokenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "exchange-github-token", // TODO find better name & group commands - hidden cmd for now.
		Short:  "Generate a Namespace Cloud token from a GitHub JWT.",
		Args:   cobra.NoArgs,
		Hidden: true,
	}

	ensuredDuration := cmd.Flags().Duration("ensure", 0, "If the current token is still valid for this duration, do nothing. Otherwise fetch a new token.")

	return fncobra.Cmd(cmd).Do(func(ctx context.Context) error {
		if !ghenv.IsRunningInActions() {
			return fnerrors.Newf("not running in a GitHub action")
		}

		if *ensuredDuration > 0 {
			var reauth *fnerrors.ReauthErr
			if err := auth.EnsureTokenValidAt(ctx, time.Now().Add(*ensuredDuration)); err == nil {
				// Token is valid for entire duration.
				return nil
			} else if !errors.As(err, &reauth) {
				// failed to load token
				return err
			}
		}

		jwt, err := github.JWT(ctx, auth.GithubJWTAudience)
		if err != nil {
			return err
		}

		res, err := fnapi.ExchangeGithubToken(ctx, jwt)
		if err != nil {
			return err
		}

		if res.Tenant != nil {
			printLoginInfo(ctx, res.Tenant)
		}

		return auth.StoreTenantToken(res.TenantToken)
	})
}

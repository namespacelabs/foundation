// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package auth

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/auth"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/github"
)

func NewExchangeGithubTokenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "exchange-github-token", // TODO find better name & group commands - hidden cmd for now.
		Short:  "Generate a Namespace Cloud token from a GitHub JWT.",
		Args:   cobra.NoArgs,
		Hidden: true,
	}

	return fncobra.Cmd(cmd).Do(func(ctx context.Context) error {
		if os.Getenv("GITHUB_ACTIONS") != "true" {
			return fnerrors.New("not running in a GitHub action")
		}

		jwt, err := github.JWT(ctx, auth.GithubJWTAudience)
		if err != nil {
			return err
		}

		res, err := fnapi.ExchangeGithubToken(ctx, jwt)
		if err != nil {
			return err
		}

		switch res.UserError {
		case "":
			tok := res.TenantToken
			// TODO remove once server migration is complete.
			if !strings.HasPrefix(tok, "nsct_") {
				tok = fmt.Sprintf("nsct_%s", res.TenantToken)
			}

			return auth.StoreTenantToken(tok)
		case "NO_INSTALLATION":
			return fnerrors.UsageError("Please follow https://github.com/apps/namespace-cloud/installations/new", "Namespace Cloud App is not installed for %q.", os.Getenv("GITHUB_REPOSITORY_OWNER"))
		default:
			return fnerrors.InvocationError("nsapi", "Failed to exchange Github token: Unknown user error %q", res.UserError)
		}
	})
}

// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package auth

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/auth"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
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
		if !github.IsRunningInActions() {
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

		if res.Tenant != nil {
			if res.Tenant.Name != "" {
				fmt.Fprintf(console.Stdout(ctx), "You are now logged into workspace %q, have a nice day.\n", res.Tenant.Name)
			}
			if res.Tenant.AppUrl != "" {
				fmt.Fprintf(console.Stdout(ctx), "You can inspect you clusters at %s\n", res.Tenant.AppUrl)
			}
		}

		return auth.StoreTenantToken(res.TenantToken)
	})
}

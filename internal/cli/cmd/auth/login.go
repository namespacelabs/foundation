// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package auth

import (
	"context"
	"fmt"

	"github.com/morikuni/aec"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"

	"namespacelabs.dev/foundation/internal/auth"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnapi"
)

func NewLoginCmd() *cobra.Command {
	var openBrowser bool

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Login to Namespace to use Ephemeral Clusters, Remote Builders, etc.",
		Args:  cobra.NoArgs,

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			res, err := fnapi.StartLogin(ctx, auth.Workspace)
			if err != nil {
				return err
			}

			stdout := console.Stdout(ctx)
			fmt.Fprintf(stdout, "%s\n", aec.Bold.Apply("Login to Namespace"))

			if openBrowser && openURL(res.LoginUrl) {
				fmt.Fprintf(stdout, "Please complete the login flow in your browser.\n\n  %s\n", res.LoginUrl)
			} else {
				fmt.Fprintf(stdout, "In order to login, open the following URL in your browser:\n\n  %s\n", res.LoginUrl)
			}

			tenant, err := completeLogin(ctx, res.LoginId)
			if err != nil {
				return err
			}

			if err := auth.StoreTenantToken(tenant.token); err != nil {
				return err
			}

			if tenant.name != "" {
				fmt.Fprintf(stdout, "\nYou are now logged into workspace %q, have a nice day.\n", tenant.name)
			} else {
				fmt.Fprintf(stdout, "\nYou are now logged in, have a nice day.\n")
			}

			return nil
		}),
	}

	cmd.Flags().BoolVar(&openBrowser, "browser", true, "Open a browser to login.")

	return cmd
}

func openURL(url string) bool {
	err := browser.OpenURL(url)
	return err == nil
}

type tenant struct {
	token string
	name  string
}

func completeLogin(ctx context.Context, id string) (tenant, error) {
	res, err := fnapi.CompleteTenantLogin(ctx, id)
	if err != nil {
		return tenant{}, err
	}

	return tenant{
		token: res.TenantToken,
		name:  res.TenantName,
	}, nil
}

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
	var (
		kind string
	)

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Login to use Namespace services (DNS and SSL management, etc).",
		Args:  cobra.NoArgs,

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			if err := auth.RemoveUser(ctx); err != nil {
				return err
			}

			if err := auth.RemoveTenantToken(ctx); err != nil {
				return err
			}

			res, err := fnapi.StartLogin(ctx, kind)
			if err != nil {
				return nil
			}

			stdout := console.Stdout(ctx)
			fmt.Fprintf(stdout, "%s\n", aec.Bold.Apply("Login to Namespace"))

			if openURL(res.LoginUrl) {
				fmt.Fprintf(stdout, "Please complete the login flow in your browser.\n\n  %s\n", res.LoginUrl)
			} else {
				fmt.Fprintf(stdout, "In order to login, open the following URL in your browser:\n\n  %s\n", res.LoginUrl)
			}

			userAuth, err := fnapi.CompleteLogin(ctx, res.LoginId, res.Kind, fnapi.TelemetryOn(ctx).GetClientID())
			if err != nil {
				return err
			}

			username, err := auth.StoreUser(ctx, userAuth)
			if err != nil {
				return err
			}

			userToken, err := auth.GenerateTokenFromUserAuth(ctx, userAuth)
			if err != nil {
				return err
			}

			tt, err := fnapi.ExchangeUserToken(ctx, userToken)
			if err != nil {
				return err
			}

			if err := auth.StoreTenantToken(tt.TenantToken); err != nil {
				return err
			}

			fmt.Fprintf(stdout, "\nHi %s, you are now logged in, have a nice day.\n", username)

			return nil
		}),
	}

	cmd.Flags().StringVar(&kind, "kind", "clerk", "Internal kind.")
	_ = cmd.Flags().MarkHidden("kind")

	return cmd
}

func openURL(url string) bool {
	err := browser.OpenURL(url)
	return err == nil
}

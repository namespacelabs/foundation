// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cmd

import (
	"context"
	"fmt"

	"github.com/morikuni/aec"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/tui"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/gitpod"
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

			auth, err := fnapi.CompleteLogin(ctx, res.LoginId, res.Kind, fnapi.TelemetryOn(ctx).GetClientID())
			if err != nil {
				return err
			}

			username, err := fnapi.StoreUser(ctx, auth)
			if err != nil {
				return err
			}

			fmt.Fprintf(stdout, "\nHi %s, you are now logged in, have a nice day.\n", username)

			return nil
		}),
	}

	cmd.Flags().StringVar(&kind, "kind", "", "Internal kind.")
	_ = cmd.Flags().MarkHidden("kind")

	cmd.AddCommand(NewRobotLogin("robot"))

	return cmd
}

func openURL(url string) bool {
	if gitpod.IsGitpod() {
		// This is to avoid using www-browser (lynx) in gitpods.
		// TODO is there a way to open a browser here?
		return false
	}

	err := browser.OpenURL(url)
	return err == nil
}

func NewRobotLogin(use string) *cobra.Command {
	robotLogin := &cobra.Command{
		Use:    use,
		Short:  "Login as a robot.",
		Args:   cobra.ExactArgs(1),
		Hidden: true,

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			accessToken, err := tui.AskSecret(ctx, "Which Access Token would you like to use today?", "That would be a Github access token.", "access token")
			if err != nil {
				return err
			}

			username, err := fnapi.LoginAsRobotAndStore(ctx, args[0], string(accessToken))
			if err != nil {
				return err
			}

			fmt.Fprintf(console.Stdout(ctx), "\nHi %s, you are now logged in, have a nice day.\n", username)
			return nil
		}),
	}

	return robotLogin
}

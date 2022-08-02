// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

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
)

const baseUrl = "https://login.namespace.so/login/cli"

func NewLoginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Login to use Namespace services (DNS and SSL management, etc).",
		Args:  cobra.NoArgs,

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			id, err := fnapi.StartLogin(ctx)
			if err != nil {
				return nil
			}

			stdout := console.Stdout(ctx)
			fmt.Fprintf(stdout, "%s\n", aec.Bold.Apply("Login to Namespace"))

			loginUrl := fmt.Sprintf("%s?id=%s", baseUrl, id)

			if err = browser.OpenURL(loginUrl); err != nil {
				fmt.Fprintf(stdout, "In order to login, open the following URL in your browser:\n\n  %s\n", loginUrl)
			} else {
				fmt.Fprintf(stdout, "Please complete the login flow in your browser.\n\n  %s\n", loginUrl)
			}

			auth, err := fnapi.CompleteLogin(ctx, id)
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

	cmd.AddCommand(NewRobotLogin("robot"))

	return cmd
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

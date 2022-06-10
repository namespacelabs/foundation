// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/tui"
	"namespacelabs.dev/foundation/internal/fnapi"
)

const loginUrl = "https://signin.prod.namespacelabs.nscloud.dev/login"

func NewLoginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Login to use Foundation services (DNS and SSL management, etc).",
		Args:  cobra.NoArgs,

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			code, err := tui.Ask(ctx, "Login to Foundation", fmt.Sprintf("In order to login, open the following URL in your browser, and then copy-paste the resulting code:\n\n  %s", loginUrl), "Code")
			if err != nil {
				return err
			}

			if code == "" {
				return context.Canceled
			}

			username, err := fnapi.StoreUser(ctx, code)
			if err != nil {
				return err
			}

			fmt.Fprintf(console.Stdout(ctx), "\nHi %s, you are now logged in, have a nice day.\n", username)
			return nil
		}),
	}

	robotLogin := &cobra.Command{
		Use:    "robot",
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

	cmd.AddCommand(robotLogin)

	return cmd
}

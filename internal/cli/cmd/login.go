// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/morikuni/aec"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/tui"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/gitpod"
)

const baseUrl = "https://login.namespace.so/login/cli"

func NewLoginCmd() *cobra.Command {
	var (
		readFromEnvVar string
	)

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Login to use Namespace services (DNS and SSL management, etc).",
		Args:  cobra.NoArgs,

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			stdout := console.Stdout(ctx)

			var auth *fnapi.UserAuth

			if readFromEnvVar != "" {
				if nsAuthJson := os.Getenv(readFromEnvVar); nsAuthJson != "" {
					auth = &fnapi.UserAuth{}
					if err := json.Unmarshal([]byte(nsAuthJson), auth); err != nil {
						return err
					}
				}
			}

			if auth == nil {
				id, exists := os.LookupEnv("nslogintoken")
				if !exists {
					var err error
					id, err = fnapi.StartLogin(ctx)
					if err != nil {
						return nil
					}

					fmt.Fprintf(stdout, "%s\n", aec.Bold.Apply("Login to Namespace"))

					loginUrl := fmt.Sprintf("%s?id=%s", baseUrl, id)

					if openURL(loginUrl) {
						fmt.Fprintf(stdout, "Please complete the login flow in your browser.\n\n  %s\n", loginUrl)
					} else {
						fmt.Fprintf(stdout, "In order to login, open the following URL in your browser:\n\n  %s\n", loginUrl)
					}
				} else {
					fmt.Fprintf(stdout, "Login pre-approved with a single-use token.\n")
				}

				tel := fnapi.TelemetryOn(ctx)
				ephemeralCliID := ""
				if tel != nil {
					ephemeralCliID = tel.GetEphemeralCliID(ctx)
				}

				var err error
				auth, err = fnapi.CompleteLogin(ctx, id, ephemeralCliID)
				if err != nil {
					return err
				}
			}

			username, err := fnapi.StoreUser(ctx, auth)
			if err != nil {
				return err
			}

			fmt.Fprintf(stdout, "\nHi %s, you are now logged in, have a nice day.\n", username)

			return nil
		}),
	}

	// This flags is used from Gitpod.
	cmd.Flags().StringVar(&readFromEnvVar, "read_from_env_var", "", "If true, try reading the auth data as JSON from this environment variable.")

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

// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

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

				var err error
				auth, err = fnapi.CompleteLogin(ctx, id, fnapi.TelemetryOn(ctx).GetID())
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

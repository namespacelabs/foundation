// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/morikuni/aec"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"

	"namespacelabs.dev/foundation/internal/auth"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

const defaultSessionDuration = 30 * 24 * time.Hour

func NewLoginCmd() *cobra.Command {
	var openBrowser, session bool
	var duration time.Duration

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Login to Namespace to access Namespace Instances, Remote Builders, etc.",
		Args:  cobra.NoArgs,

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			var sessionDuration time.Duration
			if session {
				sessionDuration = duration
			} else if duration != defaultSessionDuration {
				return fnerrors.New("--session is required when setting --duration")
			}

			res, err := fnapi.StartLogin(ctx, auth.Workspace, sessionDuration)
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

			c, err := fnapi.CompleteTenantLogin(ctx, res.LoginId)
			if err != nil {
				return err
			}

			if c.SessionToken != "" {
				if err := auth.StoreToken(auth.Token{SessionToken: c.SessionToken}); err != nil {
					return err
				}

			} else {
				if err := auth.StoreTenantToken(c.TenantToken); err != nil {
					return err
				}
			}

			if c.TenantName != "" {
				fmt.Fprintf(stdout, "\nYou are now logged into workspace %q, have a nice day.\n", c.TenantName)
			} else {
				fmt.Fprintf(stdout, "\nYou are now logged in, have a nice day.\n")
			}

			return nil
		}),
	}

	cmd.Flags().BoolVar(&openBrowser, "browser", true, "Open a browser to login.")
	cmd.Flags().BoolVar(&session, "session", true, "If set, gets a long-lived session.")
	cmd.Flags().MarkHidden("session")
	cmd.Flags().DurationVar(&duration, "duration", defaultSessionDuration, "The default duration of a session.")
	cmd.Flags().MarkHidden("duration")

	return cmd
}

func openURL(url string) bool {
	err := browser.OpenURL(url)
	return err == nil
}

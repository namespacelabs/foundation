// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"fmt"
	"net/url"

	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
)

func NewRdpCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "rdp [instance-id]",
		Short:  "Start a RDP session.",
		Args:   cobra.ArbitraryArgs,
		Hidden: true,
	}

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		cluster, err := selectClusterFriendly(ctx, args)
		if err != nil {
			return err
		}
		if cluster == nil {
			return nil
		}

		return connectToInstanceService(ctx, cluster, "rdp", func(addr string, creds *api.Cluster_ServiceState_Credentials) error {
			// Can't pass password to the RDP app on macOS:
			// https://stackoverflow.com/questions/64385564/encrypting-rdp-password-on-mac-os

			config := fmt.Sprintf("full address=s:%s", addr)
			url := "rdp://" + url.QueryEscape(config)

			fmt.Fprintf(console.Stdout(ctx), "\n")
			fmt.Fprintf(console.Stdout(ctx), "RDP Address:  %s\n", addr)
			fmt.Fprintf(console.Stdout(ctx), "RDP URL:      %s\n", url)

			if creds != nil {
				fmt.Fprintf(console.Stdout(ctx), "Username:     %s\n", creds.Username)
				fmt.Fprintf(console.Stdout(ctx), "Password:     %s\n", creds.Password)
			}
			fmt.Fprintf(console.Stdout(ctx), "\n")

			fmt.Fprintf(console.Stdout(ctx), "Opening RDP client...\n")
			return browser.OpenURL(url)
		})
	})

	return cmd
}

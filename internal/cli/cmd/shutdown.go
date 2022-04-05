// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cmd

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/provision/deploy"
)

func NewShutdownCmd() *cobra.Command {
	var (
		envRef      = "dev"
		packageName string
	)

	cmd := &cobra.Command{
		Use:   "shutdown",
		Short: "Shutdown the specified server (and dependencies).",
		Args:  cobra.ArbitraryArgs,

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			env, err := requireEnv(ctx, envRef)
			if err != nil {
				return err
			}

			locations, specified, err := allServersOrFromArgs(ctx, env, args)
			if err != nil {
				return err
			}

			packages, servers, err := loadServers(ctx, env, locations, specified)
			if err != nil {
				return err
			}

			return deploy.Shutdown(ctx, env.BindWith(packages), servers)
		}),
	}

	cmd.Flags().StringVar(&envRef, "env", "dev", "The environment to provision (as defined in the workspace).")
	cmd.Flags().StringVar(&packageName, "package_name", "", "Instead of shutting down the specified local server, shutdown the specified package resolved against the local workspace.")

	return cmd
}

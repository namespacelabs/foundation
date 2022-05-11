// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cmd

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/cmd/logs"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/runtime/kubernetes"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/module"
)

func NewLogsCmd() *cobra.Command {
	envRef := "dev"

	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Stream logs of the specified server.",
		Args:  cobra.RangeArgs(0, 1),

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			root, loc, err := module.PackageAtArgs(ctx, args)
			if err != nil {
				return err
			}

			env, err := provision.RequireEnv(root, envRef)
			if err != nil {
				return err
			}

			server, err := env.RequireServer(ctx, loc.AsPackageName())
			if err != nil {
				return err
			}

			cancel := console.SetIdleLabel(ctx, "listening for deployment changes")
			defer cancel()

			lo := logs.NewLogsObserver()
			return lo.Start(ctx, root, envRef, []*schema.Server{server.Proto()})
		}),
	}

	cmd.Flags().StringVar(&envRef, "env", envRef, "The environment to stream logs from.")
	cmd.Flags().BoolVar(&kubernetes.ObserveInitContainerLogs, "observe_init_containers", kubernetes.ObserveInitContainerLogs, "Kubernetes-specific flag to also fetch logs from init containers.")

	return cmd
}

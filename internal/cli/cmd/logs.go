// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cmd

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/logs/logtail"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/runtime/kubernetes"
)

func NewLogsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Stream logs of the specified server.",
		Args:  cobra.RangeArgs(0, 1),
	}

	cmd.Flags().BoolVar(&kubernetes.ObserveInitContainerLogs, "observe_init_containers", kubernetes.ObserveInitContainerLogs, "Kubernetes-specific flag to also fetch logs from init containers.")

	return fncobra.CmdWithServer(cmd, func(ctx context.Context, server provision.Server) error {
		console.SetIdleLabel(ctx, "listening for deployment changes")

		return logtail.Listen(ctx, server.Env(), server.Proto())
	})
}

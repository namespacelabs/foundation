// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cmd

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/logs/logtail"
	"namespacelabs.dev/foundation/runtime/kubernetes"
	"namespacelabs.dev/foundation/std/planning"
)

func NewLogsCmd() *cobra.Command {
	var (
		env     planning.Context
		locs    fncobra.Locations
		servers fncobra.Servers
	)

	return fncobra.
		Cmd(&cobra.Command{
			Use:   "logs <path/to/server>",
			Short: "Stream logs of the specified server.",
		}).
		WithFlags(func(flags *pflag.FlagSet) {
			flags.BoolVar(&kubernetes.ObserveInitContainerLogs, "observe_init_containers", kubernetes.ObserveInitContainerLogs, "Kubernetes-specific flag to also fetch logs from init containers.")
		}).
		With(
			fncobra.ParseEnv(&env),
			fncobra.ParseLocations(&locs, &env, fncobra.ParseLocationsOpts{RequireSingle: true}),
			fncobra.ParseServers(&servers, &env, &locs)).
		Do(func(ctx context.Context) error {
			console.SetIdleLabel(ctx, "listening for deployment changes")
			server := servers.Servers[0]

			return logtail.Listen(ctx, server.SealedContext(), server.Proto())
		})
}

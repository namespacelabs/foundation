// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cmd

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/cli/keyboard"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/logs/logtail"
	"namespacelabs.dev/foundation/internal/observers"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
)

func NewLogsCmd() *cobra.Command {
	var (
		env     cfg.Context
		locs    fncobra.Locations
		servers fncobra.Servers

		dump bool
	)

	return fncobra.
		Cmd(&cobra.Command{
			Use:   "logs <path/to/server>",
			Short: "Stream logs of the specified server.",
		}).
		WithFlags(func(flags *pflag.FlagSet) {
			flags.BoolVar(&kubernetes.ObserveInitContainerLogs, "observe_init_containers", kubernetes.ObserveInitContainerLogs, "Kubernetes-specific flag to also fetch logs from init containers.")
			flags.BoolVar(&dump, "dump", dump, "If set, dumps all available logs, rather than tailing the specified server.")
		}).
		With(
			fncobra.ParseEnv(&env),
			fncobra.ParseLocations(&locs, &env, fncobra.ParseLocationsOpts{RequireSingle: true}),
			fncobra.ParseServers(&servers, &env, &locs)).
		Do(func(ctx context.Context) error {
			server := servers.Servers[0]

			if dump {
				rt, err := runtime.NamespaceFor(ctx, env)
				if err != nil {
					return err
				}

				containers, err := rt.ResolveContainers(ctx, server.Proto())
				if err != nil {
					return err
				}

				if len(containers) != 1 {
					return fnerrors.InvocationError("expected a single container, got %d", len(containers))
				}

				return rt.Cluster().FetchLogsTo(ctx, console.Stdout(ctx), containers[0], runtime.FetchLogsOpts{})
			}

			event := &observers.StackUpdateEvent{
				Env: env.Environment(),
				Stack: &schema.Stack{
					Entry: []*schema.Stack_Entry{server.StackEntry()},
				},
				Focus: []string{server.Proto().PackageName},
			}

			observer := observers.Static()
			observer.PushUpdate(event)

			return keyboard.Handle(ctx, keyboard.HandleOpts{
				Provider: observer,
				Handler: func(ctx context.Context) error {
					return logtail.Listen(ctx, server.SealedContext(), server.Proto())
				},
			})
		})
}

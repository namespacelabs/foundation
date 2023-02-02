// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package debug

import (
	"context"
	"encoding/json"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/planning"
	"namespacelabs.dev/foundation/internal/planning/deploy"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
)

func newComputeConfigCmd() *cobra.Command {
	var (
		env     cfg.Context
		locs    fncobra.Locations
		servers fncobra.Servers
	)

	return fncobra.
		Cmd(&cobra.Command{
			Use:   "compute-config",
			Short: "Computes the runtime configuration of the specified server.",
		}).
		With(
			fncobra.ParseEnv(&env),
			fncobra.ParseLocations(&locs, &env, fncobra.ParseLocationsOpts{RequireSingle: true}),
			fncobra.ParseServers(&servers, &env, &locs)).
		Do(func(ctx context.Context) error {
			p, err := planning.NewPlanner(ctx, env)
			if err != nil {
				return err
			}

			plan, err := deploy.PrepareDeployServers(ctx, p, servers.Servers...)
			if err != nil {
				return err
			}

			computedPlan, err := compute.GetValue(ctx, plan)
			if err != nil {
				return err
			}

			stack := computedPlan.ComputedStack

			server := servers.Servers[0]
			ps, ok := stack.Get(server.PackageName())
			if !ok {
				return fnerrors.InternalError("expected to find %s in the stack, but didn't", server.PackageName())
			}

			out := &runtime.DeployableSpec{}
			if err := deploy.PrepareRunOpts(ctx, stack, ps, nil, out); err != nil {
				return err
			}

			j := json.NewEncoder(console.Stdout(ctx))
			j.SetIndent("", "  ")
			return j.Encode(&schema.StartupPlan{
				Args: out.MainContainer.Args,
				Env:  out.MainContainer.Env,
			})
		})
}

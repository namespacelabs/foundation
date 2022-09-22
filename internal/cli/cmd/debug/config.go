// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package debug

import (
	"context"
	"encoding/json"
	"os"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/engine/compute"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/provision/deploy"
	"namespacelabs.dev/foundation/provision/startup"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/planning"
)

func newComputeConfigCmd() *cobra.Command {
	var (
		env     planning.Context
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
			planner, err := runtime.PlannerFor(ctx, env)
			if err != nil {
				return err
			}

			server := servers.Servers[0]
			plan, err := deploy.PrepareDeployServers(ctx, env, planner, servers.Servers, nil)
			if err != nil {
				return err
			}

			computedPlan, err := compute.GetValue(ctx, plan)
			if err != nil {
				return err
			}

			stack := computedPlan.ComputedStack

			ps, ok := stack.Get(server.PackageName())
			if !ok {
				return fnerrors.InternalError("expected to find %s in the stack, but didn't", server.PackageName())
			}

			sargs := pkggraph.StartupInputs{
				Stack:         stack.Proto(),
				Server:        server.Proto(),
				ServerImage:   "imageversion",
				ServerRootAbs: server.Location.Abs(),
			}

			serverStartupPlan, err := ps.Server.Startup.EvalStartup(ctx, ps.Server.SealedContext(), sargs, nil)
			if err != nil {
				return err
			}

			c, err := startup.ComputeConfig(ctx, ps.Server.SealedContext(), serverStartupPlan, ps.ParsedDeps, sargs)
			if err != nil {
				return err
			}

			j := json.NewEncoder(os.Stdout)
			j.SetIndent("", "  ")
			return j.Encode(c)
		})
}

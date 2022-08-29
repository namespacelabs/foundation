// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package debug

import (
	"context"
	"encoding/json"
	"os"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/provision/deploy"
	"namespacelabs.dev/foundation/provision/startup"
	"namespacelabs.dev/foundation/workspace/compute"
)

func newComputeConfigCmd() *cobra.Command {
	var (
		env     provision.Env
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
			fncobra.ParseLocations(&locs, &fncobra.ParseLocationsOpts{RequireSingle: true}),
			fncobra.ParseServers(&servers, &env, &locs)).
		Do(func(ctx context.Context) error {
			server := servers.Servers[0]
			plan, err := deploy.PrepareDeployServers(ctx, env, servers.Servers, nil)
			if err != nil {
				return err
			}

			computedPlan, err := compute.GetValue(ctx, plan)
			if err != nil {
				return err
			}

			stack := computedPlan.ComputedStack

			s := stack.Get(server.PackageName())
			if s == nil {
				return fnerrors.InternalError("expected to find %s in the stack, but didn't", server.PackageName())
			}

			sargs := frontend.StartupInputs{
				Stack:         stack.Proto(),
				Server:        server.Proto(),
				ServerImage:   "imageversion",
				ServerRootAbs: server.Location.Abs(),
			}

			evald := stack.GetParsed(s.PackageName())

			serverStartupPlan, err := s.Startup.EvalStartup(ctx, env, sargs, nil)
			if err != nil {
				return err
			}
			c, err := startup.ComputeConfig(ctx, s.Env(), serverStartupPlan, evald, sargs)
			if err != nil {
				return err
			}

			j := json.NewEncoder(os.Stdout)
			j.SetIndent("", "  ")
			return j.Encode(c)
		})
}

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
	envRef := "dev"

	cmd := &cobra.Command{
		Use:   "compute-config",
		Short: "Computes the runtime configuration of the specified server.",
		Args:  cobra.RangeArgs(0, 1),

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			t, err := requireServer(ctx, args, envRef)
			if err != nil {
				return err
			}

			plan, err := deploy.PrepareDeployServers(ctx, t.Env(), []provision.Server{t}, deploy.Opts{StackOpts: deploy.StackOpts{BaseServerPort: 39999}}, nil)
			if err != nil {
				return err
			}

			computedPlan, err := compute.GetValue(ctx, plan)
			if err != nil {
				return err
			}

			stack := computedPlan.ComputedStack

			s := stack.Get(t.PackageName())
			if s == nil {
				return fnerrors.InternalError("expected to find %s in the stack, but didn't", t.PackageName())
			}

			sargs := frontend.StartupInputs{
				Stack:         stack.Proto(),
				Server:        t.Proto(),
				ServerImage:   "imageversion",
				ServerRootAbs: t.Location.Abs(),
			}

			evald := stack.GetParsed(s.PackageName())

			c, err := startup.ComputeConfig(ctx, s.Env(), evald, sargs)
			if err != nil {
				return err
			}

			j := json.NewEncoder(os.Stdout)
			j.SetIndent("", "  ")
			return j.Encode(c)
		}),
	}

	cmd.Flags().StringVar(&envRef, "env", envRef, "The environment to provision (as defined in the workspace).")

	return cmd
}

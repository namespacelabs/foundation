// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package debug

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/provision/deploy"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/module"
)

func newPrintSealedCmd() *cobra.Command {
	var (
		outputType  string = "json"
		envBound    string
		printDeploy bool
	)

	cmd := &cobra.Command{
		Use:   "print-sealed",
		Short: "Load a server definition and print it's computed sealed workspace as JSON.",
		Args:  cobra.RangeArgs(0, 1),

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			root, loc, err := module.PackageAtArgs(ctx, args)
			if err != nil {
				return err
			}

			pl := workspace.NewPackageLoader(root)

			if !printDeploy {
				sealed, err := workspace.Seal(ctx, pl, loc.AsPackageName(), nil)
				if err != nil {
					return err
				}

				return output(ctx, pl, sealed.Proto, outputType)
			} else {
				if envBound == "" {
					return fnerrors.UserError(nil, "--deploy_stack requires --env")
				}

				env, err := provision.RequireEnv(root, envBound)
				if err != nil {
					return err
				}

				t, err := env.RequireServer(ctx, loc.AsPackageName())
				if err != nil {
					return err
				}

				plan, err := deploy.PrepareDeployServers(ctx, env, []provision.Server{t}, nil)
				if err != nil {
					return err
				}

				computedPlan, err := compute.GetValue(ctx, plan)
				if err != nil {
					return err
				}

				stack := computedPlan.ComputedStack.Proto()

				for _, s := range stack.Entry {
					if err := output(ctx, t.Env(), s, outputType); err != nil {
						return err
					}
				}

				for _, endpoint := range stack.Endpoint {
					if err := output(ctx, t.Env(), endpoint, outputType); err != nil {
						return err
					}
				}

				for _, endpoint := range stack.InternalEndpoint {
					if err := output(ctx, t.Env(), endpoint, outputType); err != nil {
						return err
					}
				}

				for _, ingress := range computedPlan.IngressFragments {
					if err := output(ctx, t.Env(), ingress, outputType); err != nil {
						return err
					}
				}

				return nil
			}
		}),
	}

	cmd.Flags().StringVar(&outputType, "output", outputType, "One of json, textproto.")
	cmd.Flags().StringVar(&envBound, "env", envBound, "If specified, produce a env-bound sealed schema.")
	cmd.Flags().BoolVar(&printDeploy, "deploy_stack", false, "If specified, prints the sealed workspace for each of the servers in the stack.")

	return cmd
}

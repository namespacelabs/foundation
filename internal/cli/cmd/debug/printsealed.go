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
	"namespacelabs.dev/foundation/workspace/module"
)

func newPrintSealedCmd() *cobra.Command {
	var (
		outputType string = "json"
		envBound   string
		printStack bool
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

			if !printStack {
				sealed, err := workspace.Seal(ctx, pl, loc.AsPackageName(), nil)
				if err != nil {
					return err
				}

				return output(ctx, pl, sealed.Proto, outputType)
			} else {
				if envBound == "" {
					return fnerrors.UserError(nil, "--stack requires --env")
				}

				env, err := provision.RequireEnv(root, envBound)
				if err != nil {
					return err
				}

				t, err := env.RequireServer(ctx, loc.AsPackageName())
				if err != nil {
					return err
				}

				if printStack {
					bid := provision.NewBuildID()

					stack, err := deploy.ComputeStack(ctx, t, deploy.StackOpts{BaseServerPort: 39999}, bid)
					if err != nil {
						return err
					}

					for _, s := range stack.Servers {
						if err := output(ctx, t.Env(), s.StackEntry(), outputType); err != nil {
							return err
						}
					}

					for _, endpoint := range stack.Endpoints {
						if err := output(ctx, t.Env(), endpoint, outputType); err != nil {
							return err
						}
					}

					for _, endpoint := range stack.InternalEndpoints {
						if err := output(ctx, t.Env(), endpoint, outputType); err != nil {
							return err
						}
					}

					fragments, err := deploy.ComputeIngress(ctx, t.Env().Proto(), stack.Proto())
					if err != nil {
						return err
					}

					for _, ingress := range fragments {
						if err := output(ctx, t.Env(), ingress.WithoutAllocation(), outputType); err != nil {
							return err
						}
					}

					return nil
				} else {
					return output(ctx, t.Env(), t.StackEntry(), outputType)
				}
			}
		}),
	}

	cmd.Flags().StringVar(&outputType, "output", outputType, "One of json, textproto.")
	cmd.Flags().StringVar(&envBound, "env", envBound, "If specified, produce a env-bound sealed schema.")
	cmd.Flags().BoolVar(&printStack, "stack", false, "If specified, prints the sealed workspace for each of the servers in the stack.")

	return cmd
}

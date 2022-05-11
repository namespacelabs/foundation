// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cmd

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/runtime/endpointfwd"
	"namespacelabs.dev/foundation/internal/stack"
	"namespacelabs.dev/foundation/provision/config"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
)

func NewAttachCmd() *cobra.Command {
	envRef := "prod"
	rehydrate := true

	cmd := &cobra.Command{
		Use:   "attach",
		Short: "Attaches to the specified environment, of the specified servers.",
		Args:  cobra.ArbitraryArgs,

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			env, err := requireEnv(ctx, envRef)
			if err != nil {
				return err
			}

			serverLocs, specified, err := allServersOrFromArgs(ctx, env, args)
			if err != nil {
				return err
			}

			_, servers, err := loadServers(ctx, env, serverLocs, specified)
			if err != nil {
				return err
			}

			var stackProto *schema.Stack
			var fragment []*schema.IngressFragment

			if rehydrate {
				if len(servers) != 1 {
					return fnerrors.UserError(nil, "--rehydrate only supports a single server")
				}

				buildID, err := runtime.For(ctx, env).DeployedConfigImageID(ctx, servers[0].Proto())
				if err != nil {
					return err
				}

				rehydrated, err := config.Rehydrate(ctx, servers[0], buildID)
				if err != nil {
					return err
				}

				stackProto = rehydrated.Stack
				fragment = rehydrated.IngressFragments
			} else {
				stack, err := stack.Compute(ctx, servers, stack.ProvisionOpts{PortBase: 40000})
				if err != nil {
					return err
				}

				stackProto = stack.Proto()
				for _, entry := range stack.Proto().Entry {
					deferred, err := runtime.ComputeIngress(ctx, env.Proto(), entry, stack.Endpoints)
					if err != nil {
						return err
					}
					for _, d := range deferred {
						fragment = append(fragment, d.WithoutAllocation())
					}
				}
			}

			var focus []schema.PackageName
			for _, srv := range servers {
				focus = append(focus, srv.PackageName())
			}

			pfwd := endpointfwd.PortForward{
				LocalAddr: "localhost",
				Selector:  env,
			}

			pfwd.OnUpdate = func() {
				console.SetStickyContent(ctx, "ingress", pfwd.Render())
			}

			pfwd.Update(ctx, stackProto, focus, fragment)

			// XXX do cmd/logs too.
			select {}
		}),
	}

	cmd.Flags().StringVar(&envRef, "env", envRef, "The environment to attach to.")
	cmd.Flags().BoolVar(&rehydrate, "rehydrate", rehydrate, "If set to false, compute stack at head, rather than loading the deployed configuration.")

	return cmd
}

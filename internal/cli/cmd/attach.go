// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cmd

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/devworkflow"
	"namespacelabs.dev/foundation/devworkflow/keyboard"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/logs/logtail"
	"namespacelabs.dev/foundation/internal/observers"
	"namespacelabs.dev/foundation/internal/stack"
	"namespacelabs.dev/foundation/provision/config"
	"namespacelabs.dev/foundation/provision/deploy"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
)

func NewAttachCmd() *cobra.Command {
	h := hydrateArgs{envRef: "dev", rehydrate: true}

	cmd := &cobra.Command{
		Use:   "attach",
		Short: "Attaches to the specified environment, of the specified servers.",
		Args:  cobra.ArbitraryArgs,

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			res, err := h.ComputeStack(ctx, args)
			if err != nil {
				return err
			}

			event := &observers.StackUpdateEvent{
				Env:   res.Rehydrated.Env,
				Stack: res.Stack,
				Focus: schema.Strs(res.Focus...),
			}
			observer := observers.Static()
			observer.PushUpdate(event)

			return keyboard.Handle(ctx, keyboard.HandleOpts{
				Provider: observer,
				Keybindings: []keyboard.Handler{
					logtail.Keybinding{
						LoadEnvironment: func(name string) (runtime.Selector, error) {
							if name == res.Rehydrated.Env.Name {
								return res.Env, nil
							}

							return nil, fnerrors.InternalError("requested invalid environment: %s", name)
						},
					},
					deploy.NewNetworkPlanKeybinding("ingress"),
				},
				Handler: func(ctx context.Context) error {
					pfwd := devworkflow.NewPortFwd(ctx, nil, res.Env, "localhost")
					pfwd.OnUpdate = func() {
						event.NetworkPlan = pfwd.ToNetworkPlan()
						observer.PushUpdate(event)
					}

					pfwd.Update(res.Stack, res.Focus, res.Ingress)
					return nil
				},
			})
		}),
	}

	h.Configure(cmd)

	return cmd
}

type hydrateArgs struct {
	envRef          string
	usePackageNames bool
	rehydrate       bool

	rehydrateOnly bool
}

type hydrateResult struct {
	Env        runtime.Selector
	Stack      *schema.Stack
	Focus      []schema.PackageName
	Ingress    []*schema.IngressFragment
	Rehydrated *config.Rehydrated
}

func (h *hydrateArgs) Configure(cmd *cobra.Command) {
	cmd.Flags().StringVar(&h.envRef, "env", h.envRef, "The environment to attach to.")
	cmd.Flags().BoolVar(&h.usePackageNames, "use_package_names", h.usePackageNames, "Specify servers by using their fully qualified package name instead.")
	if !h.rehydrateOnly {
		cmd.Flags().BoolVar(&h.rehydrate, "rehydrate", h.rehydrate, "If set to false, compute stack at head, rather than loading the deployed configuration.")
	}
}

func (h *hydrateArgs) ComputeStack(ctx context.Context, args []string) (*hydrateResult, error) {
	env, err := requireEnv(ctx, h.envRef)
	if err != nil {
		return nil, err
	}

	serverLocs, specified, err := allServersOrFromArgs(ctx, env, h.usePackageNames, args)
	if err != nil {
		return nil, err
	}

	_, servers, err := loadServers(ctx, env, serverLocs, specified)
	if err != nil {
		return nil, err
	}

	var res hydrateResult
	for _, srv := range servers {
		res.Focus = append(res.Focus, srv.PackageName())
	}

	res.Env = env

	if h.rehydrate || h.rehydrateOnly {
		if len(servers) != 1 {
			return nil, fnerrors.UserError(nil, "--rehydrate only supports a single server")
		}

		buildID, err := runtime.For(ctx, env).DeployedConfigImageID(ctx, servers[0].Proto())
		if err != nil {
			return nil, err
		}

		rehydrated, err := config.Rehydrate(ctx, servers[0], buildID)
		if err != nil {
			return nil, err
		}

		res.Stack = rehydrated.Stack
		res.Ingress = rehydrated.IngressFragments
		res.Rehydrated = rehydrated
	} else {
		stack, err := stack.Compute(ctx, servers, stack.ProvisionOpts{PortRange: runtime.DefaultPortRange()})
		if err != nil {
			return nil, err
		}

		res.Stack = stack.Proto()
		for _, entry := range stack.Proto().Entry {
			deferred, err := runtime.ComputeIngress(ctx, env.Proto(), entry, stack.Endpoints)
			if err != nil {
				return nil, err
			}

			res.Ingress = deferred
		}
	}

	return &res, nil
}

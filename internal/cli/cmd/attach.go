// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cmd

import (
	"context"
	"io"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/cli/keyboard"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/logs/logtail"
	"namespacelabs.dev/foundation/internal/observers"
	"namespacelabs.dev/foundation/internal/planning/deploy/view"
	"namespacelabs.dev/foundation/internal/portforward"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/storage"
	"namespacelabs.dev/foundation/std/cfg"
)

func NewAttachCmd() *cobra.Command {
	var res hydrateResult

	return fncobra.
		Cmd(&cobra.Command{
			Use:   "attach [path/to/server]...",
			Short: "Attach the specified servers to the specified environment.",
			Args:  cobra.ArbitraryArgs}).
		With(parseHydrationWithDeps(&res, &fncobra.ParseLocationsOpts{ReturnAllIfNoneSpecified: true}, &hydrateOpts{rehydrate: true})...).
		Do(func(ctx context.Context) error {
			event := &observers.StackUpdateEvent{
				Env:              res.Env.Environment(),
				Stack:            res.Stack,
				Focus:            schema.Strs(res.Focus...),
				Deployed:         true,
				DeployedRevision: 1,
			}
			observer := observers.Static()
			observer.PushUpdate(event)

			cluster, err := runtime.NamespaceFor(ctx, res.Env)
			if err != nil {
				return err
			}

			return keyboard.Handle(ctx, keyboard.HandleOpts{
				Provider: observer,
				Keybindings: []keyboard.Handler{
					view.NetworkPlanKeybinding{Name: "ingress"},
					logtail.Keybinding{
						DefaultPaused: false,
						LoadEnvironment: func(name string) (cfg.Context, error) {
							if name == res.Env.Environment().Name {
								return res.Env, nil
							}

							return nil, fnerrors.InternalError("requested invalid environment: %s", name)
						},
					},
				},
				Handler: func(ctx context.Context) error {
					pfwd := &portforward.PortForward{
						Env:       res.Env.Environment(),
						LocalAddr: "localhost",
						Debug:     console.Debug(ctx),
						Warnings:  console.Warnings(ctx),
						ForwardPort: func(server runtime.Deployable, port int32, localAddr []string, callback runtime.SinglePortForwardedFunc) (io.Closer, error) {
							return cluster.ForwardPort(ctx, server, port, localAddr, callback)
						},
						ForwardIngress: func(localAddr []string, port int, callback runtime.PortForwardedFunc) (io.Closer, error) {
							return cluster.Cluster().ForwardIngress(ctx, localAddr, port, callback)
						},
					}
					pfwd.OnUpdate = func(plan *storage.NetworkPlan) {
						event := protos.Clone(event)
						event.NetworkPlan = plan
						observer.PushUpdate(event)
					}

					pfwd.Update(res.Stack, res.Focus, res.Ingress)

					return nil
				},
			})
		})
}

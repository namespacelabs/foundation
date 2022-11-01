// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cmd

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/cli/keyboard"
	"namespacelabs.dev/foundation/internal/devsession"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/logs/logtail"
	"namespacelabs.dev/foundation/internal/observers"
	"namespacelabs.dev/foundation/internal/planning/deploy/view"
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
			Short: "Attaches to the specified environment, of the specified servers.",
			Args:  cobra.ArbitraryArgs}).
		With(parseHydrationWithDeps(&res, &fncobra.ParseLocationsOpts{ReturnAllIfNoneSpecified: true}, &hydrateOpts{rehydrate: true})...).
		Do(func(ctx context.Context) error {
			event := &observers.StackUpdateEvent{
				Env:   res.Env.Environment(),
				Stack: res.Stack,
				Focus: schema.Strs(res.Focus...),
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
					view.NewNetworkPlanKeybinding("ingress"),
					logtail.NewKeybinding(func(name string) (cfg.Context, error) {
						if name == res.Env.Environment().Name {
							return res.Env, nil
						}

						return nil, fnerrors.InternalError("requested invalid environment: %s", name)
					}),
				},
				Handler: func(ctx context.Context) error {
					pfwd := devsession.NewPortFwd(ctx, nil, res.Env, cluster, "localhost")
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

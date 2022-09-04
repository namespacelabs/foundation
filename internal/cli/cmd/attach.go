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
	"namespacelabs.dev/foundation/provision/deploy/view"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
)

func NewAttachCmd() *cobra.Command {
	var res hydrateResult

	return fncobra.
		Cmd(&cobra.Command{
			Use:   "attach [path/to/server]...",
			Short: "Attaches to the specified environment, of the specified servers.",
			Args:  cobra.ArbitraryArgs}).
		With(parseHydrationWithDeps(&res, &fncobra.ParseLocationsOpts{DefaultToAllWhenEmpty: true}, &hydrateOpts{rehydrate: true})...).
		Do(func(ctx context.Context) error {
			event := &observers.StackUpdateEvent{
				Env:   res.Env.Environment(),
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
							if name == res.Env.Environment().Name {
								return res.Env, nil
							}

							return nil, fnerrors.InternalError("requested invalid environment: %s", name)
						},
					},
					view.NewNetworkPlanKeybinding("ingress"),
				},
				Handler: func(ctx context.Context) error {
					pfwd := devworkflow.NewPortFwd(ctx, nil, res.Env, "localhost")
					pfwd.OnUpdate = func() {
						event.NetworkPlan, _ = pfwd.ToNetworkPlan()
						observer.PushUpdate(event)
					}

					pfwd.Update(res.Stack, res.Focus, res.Ingress)
					return nil
				},
			})
		})
}

// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cmd

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
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
	var servingAddr string

	return fncobra.
		Cmd(&cobra.Command{
			Use:   "attach [path/to/server]...",
			Short: "Attaches to the specified environment, of the specified servers.",
			Args:  cobra.ArbitraryArgs}).
		WithFlags(func(flags *pflag.FlagSet) {
			flags.StringVarP(&servingAddr, "listen", "H", "", "webui: listen on the specified address.")
		}).
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

type attachSessionLike struct {
	cluster runtime.ClusterNamespace
	stack   *schema.Stack
	event   *observers.StackUpdateEvent
}

func (s attachSessionLike) ResolveServer(ctx context.Context, serverID string) (runtime.ClusterNamespace, runtime.Deployable, error) {
	entry := s.stack.GetServerByID(serverID)
	if entry != nil {
		return s.cluster, entry.Server, nil
	}

	return nil, nil, fnerrors.Newf("%s: no such server in the current session", serverID)
}

func (s attachSessionLike) NewClient(needsHistory bool) (devsession.ObserverLike, error) {
	ch := make(chan *devsession.Update, 1)
	ch <- &devsession.Update{
		StackUpdate: &devsession.Stack{
			Revision:    s.event.DeployedRevision,
			Env:         s.event.Env,
			Stack:       s.event.Stack,
			Focus:       s.event.Focus,
			NetworkPlan: s.event.NetworkPlan,
			Deployed:    s.event.Deployed,
		},
	}

	return attachedObserver{ch}, nil
}

func (s attachSessionLike) DeferRequest(req *devsession.DevWorkflowRequest) {
	// Ignoring.
}

type attachedObserver struct {
	ch chan *devsession.Update
}

func (a attachedObserver) Events() chan *devsession.Update {
	return a.ch
}

func (a attachedObserver) Close() {
	// XXX
}

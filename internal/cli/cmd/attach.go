// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cmd

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/cli/keyboard"
	"namespacelabs.dev/foundation/internal/devsession"
	"namespacelabs.dev/foundation/internal/executor"
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
	var useWebUI bool

	return fncobra.
		Cmd(&cobra.Command{
			Use:   "attach [path/to/server]...",
			Short: "Attaches to the specified environment, of the specified servers.",
			Args:  cobra.ArbitraryArgs}).
		WithFlags(func(flags *pflag.FlagSet) {
			flags.StringVarP(&servingAddr, "listen", "H", "", "webui: listen on the specified address.")
			flags.BoolVar(&useWebUI, "webui", useWebUI, "If true, exposes a web UI as well.")
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
						DefaultPaused: useWebUI,
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

					if !useWebUI {
						return nil
					}

					lis, err := startListener(servingAddr)
					if err != nil {
						return fnerrors.InternalError("Failed to start listener at %q: %w", servingAddr, err)
					}

					defer lis.Close()

					r := mux.NewRouter()
					fncobra.RegisterPprof(r)
					devsession.RegisterSomeEndpoints(attachSessionLike{cluster, res.Stack, event}, r)

					mux, err := devsession.PrebuiltWebUI(ctx)
					if err != nil {
						return err
					}

					r.PathPrefix("/").Handler(mux)

					srv := &http.Server{
						Handler:      r,
						Addr:         servingAddr,
						WriteTimeout: 15 * time.Second,
						ReadTimeout:  15 * time.Second,
						BaseContext:  func(l net.Listener) context.Context { return ctx },
					}

					eg := executor.New(ctx, "attach")

					eg.Go(func(ctx context.Context) error {
						// On cancelation, i.e. Ctrl-C, ask the server to shutdown. This will lead to the next go-routine below, actually returns.
						<-ctx.Done()

						ctxT, cancelT := context.WithTimeout(ctx, 1*time.Second)
						defer cancelT()

						return srv.Shutdown(ctxT)
					})

					eg.Go(func(ctx context.Context) error {
						updateWebUISticky(ctx, "running at: http://%s", lis.Addr())
						return srv.Serve(lis)
					})

					return eg.Wait()
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

	return nil, nil, fnerrors.New("%s: no such server in the current session", serverID)
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

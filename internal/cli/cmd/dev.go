// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cmd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra/planningargs"
	"namespacelabs.dev/foundation/internal/cli/keyboard"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/devsession"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/logs/logtail"
	"namespacelabs.dev/foundation/internal/planning/deploy/view"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/tasks"
)

func NewDevCmd() *cobra.Command {
	var (
		servingAddr string
		env         cfg.Context
		locs        fncobra.Locations
		servers     planningargs.Servers
	)

	return fncobra.
		Cmd(&cobra.Command{
			Use:   "dev <path/to/server>...",
			Short: "Starts a development session, continuously building and deploying a server.",
			Args:  cobra.MinimumNArgs(1),
		}).
		WithFlags(func(flags *pflag.FlagSet) {
			flags.StringVarP(&servingAddr, "listen", "H", "", "Listen on the specified address.")
		}).
		With(
			fncobra.ParseEnv(&env),
			fncobra.ParseLocations(&locs, &env),
			planningargs.ParseServers(&servers, &env, &locs)).
		Do(func(ctx context.Context) error {
			ctx, sink := tasks.WithStatefulSink(ctx)

			return compute.Do(ctx, func(ctx context.Context) error {
				lis, err := startListener(servingAddr)
				if err != nil {
					return fnerrors.InternalError("Failed to start listener at %q: %w", servingAddr, err)
				}

				defer lis.Close()

				var serverPackages []string
				for _, s := range servers.Servers {
					serverPackages = append(serverPackages, s.PackageName().String())
				}

				localHost := lis.Addr().(*net.TCPAddr).IP.String()

				sesh, err := devsession.NewSession(console.Errors(ctx), sink, localHost,
					schema.SpecToEnv(cfg.EnvsOrDefault(locs.Root.DevHost(), locs.Root.Workspace().Proto())...))
				if err != nil {
					return err
				}

				// Kick off the dev workflow.
				sesh.DeferRequest(&devsession.DevWorkflowRequest{
					Type: &devsession.DevWorkflowRequest_SetWorkspace_{
						SetWorkspace: &devsession.DevWorkflowRequest_SetWorkspace{
							AbsRoot:           env.Workspace().LoadedFrom().AbsPath,
							PackageName:       serverPackages[0],
							AdditionalServers: serverPackages[1:],
							EnvName:           env.Environment().Name,
						},
					},
				})

				return keyboard.Handle(ctx, keyboard.HandleOpts{
					Provider: sesh,
					Keybindings: []keyboard.Handler{
						view.NetworkPlanKeybinding{Name: "stack"},
						logtail.Keybinding{
							LoadEnvironment: func(envName string) (cfg.Context, error) {
								return cfg.LoadContext(locs.Root, envName)
							},
						},
					},
					Handler: func(ctx context.Context) error {
						r := mux.NewRouter()
						fncobra.RegisterPprof(r)
						devsession.RegisterEndpoints(sesh, r)

						srv := &http.Server{
							Handler:      r,
							Addr:         servingAddr,
							WriteTimeout: 15 * time.Second,
							ReadTimeout:  15 * time.Second,
							BaseContext:  func(l net.Listener) context.Context { return ctx },
						}

						return sesh.Run(ctx, func(eg *executor.Executor) {
							eg.Go(func(ctx context.Context) error {
								// On cancelation, i.e. Ctrl-C, ask the server to shutdown. This will lead to the next go-routine below, actually returns.
								<-ctx.Done()

								ctxT, cancelT := context.WithTimeout(ctx, 1*time.Second)
								defer cancelT()

								return srv.Shutdown(ctxT)
							})

							eg.Go(func(ctx context.Context) error {
								return srv.Serve(lis)
							})
						})
					},
				})
			})
		},
		)
}

func startListener(specified string) (net.Listener, error) {
	const defaultHostname = "127.0.0.1"
	const defaultStartingPort = 4001

	if specified != "" {
		return net.Listen("tcp", specified)
	}

	for port := defaultStartingPort; ; port++ {
		l, err := net.Listen("tcp", fmt.Sprintf("%s:%d", defaultHostname, port))
		if err != nil {
			var errno syscall.Errno
			if errors.As(err, &errno) {
				if errno == syscall.EADDRINUSE {
					continue
				}
			}
			return nil, err
		}

		return l, nil
	}
}

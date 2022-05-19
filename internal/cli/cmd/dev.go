// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

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
	"github.com/morikuni/aec"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/build/binary"
	"namespacelabs.dev/foundation/devworkflow"
	"namespacelabs.dev/foundation/internal/cli/cmd/logs"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/reverseproxy"
	"namespacelabs.dev/foundation/languages/web"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/module"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func NewDevCmd() *cobra.Command {
	var (
		envRef       = "dev"
		servingAddr  string
		devWebServer = false
	)

	const webPackage schema.PackageName = "namespacelabs.dev/foundation/devworkflow/web"

	cmd := &cobra.Command{
		Use:   "dev",
		Short: "Starts a development session, continuously building and deploying a server.",
		Args:  cobra.MinimumNArgs(1),

		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, sink := tasks.WithStatefulSink(cmd.Context())

			ctxWithCancel, cancel := fncobra.WithSigIntCancel(ctx)
			defer cancel()

			return compute.Do(ctxWithCancel, func(ctx context.Context) error {
				root, err := module.FindRoot(ctx, ".")
				if err != nil {
					return err
				}

				lis, err := startListener(servingAddr)
				if err != nil {
					return err
				}

				defer lis.Close()

				pl := workspace.NewPackageLoader(root)

				var serverPackages []string
				var serverProtos []*schema.Server
				for _, p := range args {
					parsed, err := pl.LoadByName(ctx, root.RelPackage(p).AsPackageName())
					if err != nil {
						return err
					}

					if parsed.Server == nil {
						return fnerrors.UserError(parsed.Location, "`fn dev` works exclusively with servers (for now)")
					}

					serverPackages = append(serverPackages, parsed.PackageName().String())
					serverProtos = append(serverProtos, parsed.Server)
				}

				inputHandler := logs.NewTerm()

				// This has to happen before new stackState gets created to render commands at the top.
				inputHandler.SetConsoleSticky(ctx)

				localHost := lis.Addr().(*net.TCPAddr).IP.String()

				stackState, err := devworkflow.NewSession(ctx, sink, localHost,
					fmt.Sprintf(" %s: web ui running at: http://%s",
						aec.Bold.Apply("Foundation"), lis.Addr()))
				if err != nil {
					return err
				}
				defer stackState.Close()

				go func() {
					_ = stackState.Run(ctx)
				}()

				// Kick off the dev workflow.
				stackState.Ch <- &devworkflow.DevWorkflowRequest{
					Type: &devworkflow.DevWorkflowRequest_SetWorkspace_{
						SetWorkspace: &devworkflow.DevWorkflowRequest_SetWorkspace{
							AbsRoot:           root.Abs(),
							PackageName:       serverPackages[0],
							AdditionalServers: serverPackages[1:],
							EnvName:           envRef,
						},
					},
				}

				r := mux.NewRouter()
				srv := &http.Server{
					Handler:      r,
					Addr:         servingAddr,
					WriteTimeout: 15 * time.Second,
					ReadTimeout:  15 * time.Second,
					BaseContext:  func(l net.Listener) context.Context { return ctx },
				}

				shutdownErr := make(chan error)

				fncobra.RegisterPprof(r)
				devworkflow.RegisterEndpoints(stackState, r)

				ch, done := stackState.NewClient()
				defer done()

				go inputHandler.HandleEvents(ctx, root, serverProtos, cancel, ch)

				if devWebServer {
					localPort := lis.Addr().(*net.TCPAddr).Port
					webPort := localPort + 1
					proxyTarget, err := web.StartDevServer(ctx, root, webPackage, localPort, webPort)
					if err != nil {
						return err
					}
					r.PathPrefix("/").Handler(reverseproxy.Make(proxyTarget, reverseproxy.DefaultLocalProxy()))
				} else {
					dev, err := provision.RequireEnv(root, "dev")
					if err != nil {
						return err
					}

					pkg, err := workspace.NewPackageLoader(root).LoadByName(ctx, webPackage)
					if err != nil {
						return err
					}

					imagePlan, err := binary.PlanImage(ctx, pkg, dev, true, nil)
					if err != nil {
						return err
					}

					// A build is triggered here, but in fact this will most times just do a cache hit.
					mux, err := compute.GetValue(ctx, web.ServeFS(imagePlan.Image, true))
					if err != nil {
						return err
					}

					r.PathPrefix("/").Handler(mux)
				}

				go func() {
					// On cancelation, i.e. Ctrl-C, ask the server to shutdown.
					<-ctx.Done()
					ctxT, cancelT := context.WithTimeout(ctx, 1*time.Second)
					defer cancelT()

					shutdownErr <- srv.Shutdown(ctxT)
				}()

				if err := srv.ListenAndServe(); err != nil {
					if err != http.ErrServerClosed {
						// Fetch logs here
						return err
					}
				}

				// Wait for shutdown.
				return <-shutdownErr
			})
		},
	}

	cmd.Flags().StringVarP(&servingAddr, "listen", "H", "", "Listen on the specified address.")
	cmd.Flags().StringVar(&envRef, "env", envRef, "The environment to provision (as defined in the workspace).")
	cmd.Flags().BoolVar(&devWebServer, "devweb", devWebServer, "Whether to start a development web frontend.")
	cmd.Flags().BoolVar(&devworkflow.AlsoOutputBuildToStderr, "alsooutputtostderr", devworkflow.AlsoOutputBuildToStderr, "Also send build output to stderr.")

	return cmd
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

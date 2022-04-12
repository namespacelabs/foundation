// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cmd

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/module"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func NewLogsCmd() *cobra.Command {
	envRef := "dev"

	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Stream logs of the specified server.",
		Args:  cobra.RangeArgs(0, 1),

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			root, loc, err := module.PackageAtArgs(ctx, args)
			if err != nil {
				return err
			}

			env, err := provision.RequireEnv(root, envRef)
			if err != nil {
				return err
			}

			server, err := env.RequireServer(ctx, loc.AsPackageName())
			if err != nil {
				return err
			}

			rt := runtime.For(env)

			streams := map[string]*logStream{}
			var mu sync.Mutex

			cancel := tasks.SetIdleLabel(ctx, "listening for deployment changes")
			defer cancel()

			return rt.Observe(ctx, server.Proto(), runtime.ObserveOpts{}, func(ev runtime.ObserveEvent) error {
				mu.Lock()
				existing := streams[ev.InstanceID]
				if ev.Removed {
					delete(streams, ev.InstanceID)
				}
				mu.Unlock()

				if ev.Added {
					if existing != nil {
						return nil
					}
				} else if ev.Removed {
					if existing != nil {
						existing.cancel()
					}
					return nil
				}

				newS := &logStream{}
				mu.Lock()
				streams[ev.InstanceID] = newS
				mu.Unlock()

				compute.On(ctx).Detach(tasks.Action("stream-log").Indefinite(), func(ctx context.Context) error {
					w := console.Output(ctx, ev.HumanReadableID)
					ctx, cancel := context.WithCancel(ctx)

					if !newS.set(cancel, w) {
						// Raced with pod disappearing.
						return nil
					}

					fmt.Fprintln(w, "<Starting to stream>")

					return rt.StreamLogsTo(ctx, w, server.Proto(), runtime.StreamLogsOpts{
						InstanceID: ev.InstanceID,
						TailLines:  20,
						Follow:     true,
					})
				})

				return nil
			})
		}),
	}

	cmd.Flags().StringVar(&envRef, "env", envRef, "The environment to stream logs from.")
	cmd.Flags().BoolVar(&kubernetes.ObserveInitContainerLogs, "observe_init_containers", kubernetes.ObserveInitContainerLogs, "Kubernetes-specific flag to also fetch logs from init containers.")

	return cmd
}

type logStream struct {
	mu         sync.Mutex
	cancelFunc func()
	cancelled  bool
	w          io.Writer
}

func (ls *logStream) cancel() {
	ls.mu.Lock()
	cancel := ls.cancelFunc
	ls.cancelFunc = nil
	wasCancelled := ls.cancelled
	ls.cancelled = true
	w := ls.w
	ls.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	if !wasCancelled {
		fmt.Fprintln(w, "<Closed>")
	}
}

func (ls *logStream) set(cancel func(), w io.Writer) bool {
	ls.mu.Lock()
	cancelled := ls.cancelled
	ls.cancelFunc = cancel
	ls.w = w
	ls.mu.Unlock()
	return !cancelled
}

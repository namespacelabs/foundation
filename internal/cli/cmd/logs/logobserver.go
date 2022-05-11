// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package logs

import (
	"context"
	"fmt"
	"io"
	"sync"

	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

type LogsObserver interface {
	// Start fetching logs. Outputs to the console.
	Start(ctx context.Context, root *workspace.Root, envRef string, servers []*schema.Server) error
	Stop()
}

// Returns non-thread-safe LogsObserver.
func NewLogsObserver() LogsObserver {
	return &logsObserver{}
}

type logsObserver struct {
	cancel context.CancelFunc
}

func (lo *logsObserver) Start(ctx context.Context, root *workspace.Root, envRef string, server []*schema.Server) error {
	env, err := provision.RequireEnv(root, envRef)
	if err != nil {
		return err
	}
	rt := runtime.For(ctx, env)
	ctxWithCancel, cancel := context.WithCancel(ctx)
	lo.cancel = cancel
	for _, server := range server {
		server := server
		go func() {
			if err := startSingle(ctxWithCancel, rt, server); err != nil {
				fmt.Fprintf(console.Errors(ctx), "Error while observing logs: %v", err)
			}
		}()
	}
	return nil
}

func (lo *logsObserver) Stop() {
	if lo.cancel != nil {
		lo.cancel()
	}
}

func startSingle(ctx context.Context, rt runtime.Runtime, server *schema.Server) error {
	var mu sync.Mutex
	streams := map[string]*logStream{}
	return rt.Observe(ctx, server, runtime.ObserveOpts{}, func(ev runtime.ObserveEvent) error {
		mu.Lock()
		existing := streams[ev.ContainerReference.UniqueID()]
		if ev.Removed {
			delete(streams, ev.ContainerReference.UniqueID())
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
		streams[ev.ContainerReference.UniqueID()] = newS
		mu.Unlock()

		compute.On(ctx).Detach(tasks.Action("stream-log").Indefinite(), func(ctx context.Context) error {
			w := console.Output(ctx, ev.HumanReadableID)
			ctx, cancel := context.WithCancel(ctx)

			if !newS.set(cancel, w) {
				// Raced with pod disappearing.
				return nil
			}

			fmt.Fprintln(w, "<Starting to stream>")

			return rt.FetchLogsTo(ctx, w, ev.ContainerReference, runtime.FetchLogsOpts{
				TailLines: 20,
				Follow:    true,
			})
		})

		return nil
	})
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

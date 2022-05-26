// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package logtail

import (
	"context"
	"fmt"
	"io"
	"sync"

	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

// Listen blocks fetching logs from a container.
func Listen(ctx context.Context, env runtime.Selector, server *schema.Server) error {
	// TODO simplify runtime creation.
	rt := runtime.For(ctx, env)
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

			fmt.Fprintf(w, "<Starting log tail for %s>\n", ev.HumanReadableID)

			return rt.FetchLogsTo(ctx, w, ev.ContainerReference, runtime.FetchLogsOpts{
				TailLines: 30,
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

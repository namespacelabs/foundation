// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package logtail

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/devworkflow/keyboard"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

type Keybinding struct {
	LoadEnvironment func(name string) (runtime.Selector, error)
}

type logState struct {
	Revision string // Revisions are used to track whether the log state is still required.

	PackageName schema.PackageName
	Cancel      func()
}

func (l Keybinding) Key() string { return "l" }
func (l Keybinding) Label(enabled bool) string {
	if !enabled {
		return "stream logs"
	}
	return "pause logs " // Additional space at the end for a better allignment.
}

func (l Keybinding) Handle(ctx context.Context, ch chan keyboard.Event, control chan<- keyboard.Control) {
	logging := false

	var previousStack *schema.Stack
	var previousEnv string
	var previousFocus []string

	// This map keeps track of which servers we're streaming logs for, keyed
	// also by environment. This leads to a natural cancelation of servers that
	// are no longer in the stack, or as part of an environment change.
	listening := map[string]*logState{} // `{env}/{package}` --> LogState

	for event := range ch {
		newStack := previousStack
		newEnv := previousEnv
		newFocus := previousFocus

		switch event.Operation {
		case keyboard.OpSet:
			logging = event.Enabled

		case keyboard.OpStackUpdate:
			newStack = event.StackUpdate.Stack
			newEnv = event.StackUpdate.Env.GetName()
			newFocus = event.StackUpdate.Focus
		}

		if logging {
			for _, server := range newStack.GetEntry() {
				if slices.Index(newFocus, server.Server.PackageName) < 0 {
					continue
				}

				key := fmt.Sprintf("%s[%s]", server.Server.PackageName, newEnv)
				if previous, has := listening[key]; has {
					previous.Revision = event.EventID
				} else {
					fmt.Fprintf(console.Output(ctx, "logs"), "starting log for %s\n", key)

					// Start logging.
					ctxWithCancel, cancelF := context.WithCancel(ctx)
					listening[key] = &logState{
						Revision:    event.EventID,
						PackageName: server.GetPackageName(),
						Cancel:      cancelF,
					}

					server := server.Server // Capture server.
					go func() {
						env, err := l.LoadEnvironment(newEnv)
						if err == nil {
							err = Listen(ctxWithCancel, env, server)
						}

						if err != nil && !errors.Is(err, context.Canceled) {
							fmt.Fprintf(console.Errors(ctx), "Error starting logs: %v\n", err)
						}
					}()
				}
			}
		}

		var keys []string
		for key, l := range listening {
			if l.Revision != event.EventID {
				keys = append(keys, key)
				l.Cancel()
				delete(listening, key)
			}
		}

		if len(keys) > 0 {
			fmt.Fprintf(console.Stderr(ctx), "Stopped listening to logs of: %s\n", strings.Join(keys, ", "))
		}

		previousStack = newStack
		previousEnv = newEnv
		previousFocus = newFocus

		switch event.Operation {
		case keyboard.OpSet:
			c := keyboard.Control{Operation: keyboard.ControlAck}
			c.AckEvent.HandlerID = event.HandlerID
			c.AckEvent.EventID = event.EventID

			control <- c
		}
	}

	for _, l := range listening {
		l.Cancel()
	}
}

// Listen blocks fetching logs from a container.
func Listen(ctx context.Context, env runtime.Selector, server *schema.Server) error {
	// TODO simplify runtime creation.
	rt := runtime.For(ctx, env)
	var mu sync.Mutex
	streams := map[string]*logStream{}
	return rt.Observe(ctx, server, runtime.ObserveOpts{}, func(ev runtime.ObserveEvent) error {
		mu.Lock()
		existing := streams[ev.ContainerReference.UniqueId]
		if ev.Removed {
			delete(streams, ev.ContainerReference.UniqueId)
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
		streams[ev.ContainerReference.UniqueId] = newS
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

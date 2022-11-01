// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package logtail

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/internal/cli/keyboard"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/tasks"
)

type Keybinding struct {
	LoadEnvironment func(name string) (cfg.Context, error)

	enableLogs                         bool
	logsHaveBeenEnabledAfterDeployment bool
}

func NewKeybinding(loadEnvironment func(name string) (cfg.Context, error)) *Keybinding {
	return &Keybinding{
		LoadEnvironment: loadEnvironment,
	}
}

type logState struct {
	Revision string // Revisions are used to track whether the log state is still required.

	PackageName schema.PackageName
	Cancel      func()
}

func (l *Keybinding) Key() string { return "l" }
func (l *Keybinding) Label() string {
	if l.enableLogs {
		return "pause logs " // Additional space at the end for a better allignment.
	}
	return "stream logs"
}

func (l *Keybinding) Handle(ctx context.Context, ch chan keyboard.Event, control chan<- keyboard.Control) {
	var previousStack *schema.Stack
	var previousEnv string
	var previousFocus []string

	// This map keeps track of which servers we're streaming logs for, keyed
	// also by environment. This leads to a natural cancelation of servers that
	// are no longer in the stack, or as part of an environment change.
	listening := map[string]*logState{} // `{env}/{package}` --> LogState

	out := console.TypedOutput(ctx, "server-logs", console.CatOutputUs)

	for event := range ch {
		newStack := previousStack
		newEnv := previousEnv
		newFocus := previousFocus

		switch event.Operation {
		case keyboard.OpToggle:
			l.enableLogs = !l.enableLogs

		case keyboard.OpStackUpdate:
			newStack = event.StackUpdate.Stack
			newEnv = event.StackUpdate.Env.GetName()
			newFocus = event.StackUpdate.Focus

			if event.StackUpdate.NetworkPlan != nil && event.StackUpdate.NetworkPlan.IsDeploymentFinished() && !l.logsHaveBeenEnabledAfterDeployment {
				l.enableLogs = true
				// Auto-enable logs after the first deployment.
				l.logsHaveBeenEnabledAfterDeployment = true
			}
		}

		style := colors.Ctx(ctx)

		if l.enableLogs {
			for _, server := range newStack.GetEntry() {
				if slices.Index(newFocus, server.Server.PackageName) < 0 {
					continue
				}

				key := fmt.Sprintf("%s[%s]", server.Server.PackageName, newEnv)
				if previous, has := listening[key]; has {
					previous.Revision = event.EventID
				} else {
					// Start logging.
					ctxWithCancel, cancelF := context.WithCancel(ctx)
					listening[key] = &logState{
						Revision:    event.EventID,
						PackageName: server.GetPackageName(),
						Cancel:      cancelF,
					}

					server := server.Server // Capture server.
					go func() {
						// There is a race with the "Network plan" block also receiving the same event,
						// and we want to log this message after the network plan has been printed,
						// so doing it in a goroutine.
						fmt.Fprintf(out, "%s %s\n", style.Header.Apply("Starting log for"), style.LogArgument.Apply(key))

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
			fmt.Fprintf(out, "%s %s\n", style.Header.Apply("Stopped listening to logs of:"), style.LogArgument.Apply(strings.Join(keys, ", ")))
		}

		previousStack = newStack
		previousEnv = newEnv
		previousFocus = newFocus

		switch event.Operation {
		case keyboard.OpToggle:
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
func Listen(ctx context.Context, env cfg.Context, server runtime.Deployable) error {
	// TODO simplify runtime creation.
	rt, err := runtime.NamespaceFor(ctx, env)
	if err != nil {
		return err
	}

	var mu sync.Mutex
	streams := map[string]*logStream{}
	return rt.Observe(ctx, server, runtime.ObserveOpts{}, func(ev runtime.ObserveEvent) (bool, error) {
		mu.Lock()
		existing := streams[ev.ContainerReference.UniqueId]
		if ev.Removed {
			delete(streams, ev.ContainerReference.UniqueId)
		}
		mu.Unlock()

		if ev.Added {
			if existing != nil {
				return false, nil
			}
		} else if ev.Removed {
			if existing != nil {
				existing.cancel()
			}
			return false, nil
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

			style := colors.Ctx(ctx)
			fmt.Fprintf(w, "\n%s", style.Comment.Apply("──────────"))
			fmt.Fprint(w, style.Highlight.Apply(fmt.Sprintf(" Log tail for %s ", ev.HumanReadableID)))
			fmt.Fprintf(w, "%s\n\n", style.Comment.Apply("──────────"))

			return rt.Cluster().FetchLogsTo(ctx, w, ev.ContainerReference, runtime.FetchLogsOpts{
				TailLines: 30,
				Follow:    true,
			})
		})

		return false, nil
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

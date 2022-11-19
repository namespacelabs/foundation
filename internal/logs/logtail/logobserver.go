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

	"go.uber.org/atomic"
	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/internal/cli/keyboard"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/schema"
	runtimepb "namespacelabs.dev/foundation/schema/runtime"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/tasks"
)

type Keybinding struct {
	LoadEnvironment func(name string) (cfg.Context, error)
}

type logState struct {
	Revision string // Revisions are used to track whether the log state is still required.

	PackageName schema.PackageName
	Cancel      func()
}

func (l Keybinding) Key() string { return "l" }

func (l Keybinding) States() []keyboard.HandlerState {
	return []keyboard.HandlerState{
		{State: "logging", Label: "pause logs "}, // Additional space at the end for a better allignment.
		{State: "notlogging", Label: "stream logs"},
	}
}

func (l Keybinding) Handle(ctx context.Context, ch chan keyboard.Event, control chan<- keyboard.Control) {
	defer close(control)

	logging := true

	var previousStack *schema.Stack
	var previousEnv string
	var previousFocus []string
	var previousDeployed bool

	// This map keeps track of which servers we're streaming logs for, keyed
	// also by environment. This leads to a natural cancelation of servers that
	// are no longer in the stack, or as part of an environment change.
	listening := map[string]*logState{} // `{env}/{package}` --> LogState

	out := console.TypedOutput(ctx, "server-logs", console.CatOutputUs)

	for event := range ch {
		newStack := previousStack
		newEnv := previousEnv
		newFocus := previousFocus
		newDeployed := previousDeployed

		switch event.Operation {
		case keyboard.OpSet:
			logging = event.CurrentState == "logging"

		case keyboard.OpStackUpdate:
			newStack = event.StackUpdate.Stack
			newEnv = event.StackUpdate.Env.GetName()
			newFocus = event.StackUpdate.Focus
			newDeployed = event.StackUpdate.Deployed

		default:
			continue
		}

		style := colors.Ctx(ctx)

		// Only start streaming after getting a Deployed signal, so the UX is better.
		if logging && newDeployed {
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
						var once sync.Once

						env, err := l.LoadEnvironment(newEnv)
						if err == nil {
							var containerCount atomic.Int32
							err = Listen(ctxWithCancel, out, env, server, func(ev runtime.ObserveEvent) io.Writer {
								return &writerWithHeader{
									onStart: func(w io.Writer) {
										once.Do(func() {
											fmt.Fprintf(out, "%s %s\n", ">>> Logging", style.LogArgument.Apply(key))
										})

										if containerCount.Inc() > 1 {
											fmt.Fprintf(out, "%s\n", style.Comment.Apply("You may still observe logs of previous instances of the same server."))
											fmt.Fprint(w, style.Comment.Apply(fmt.Sprintf("Log tail for %s", humanReadable(ev))))
											fmt.Fprintln(w)
										}
									},
									w: console.Output(ctx, humanReadable(ev)),
								}
							})
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
			fmt.Fprintf(out, "%s %s\n", "<<< No longer logging", style.LogArgument.Apply(strings.Join(keys, ", ")))
		}

		previousStack = newStack
		previousEnv = newEnv
		previousFocus = newFocus
		previousDeployed = newDeployed

		switch event.Operation {
		case keyboard.OpSet:
			c := keyboard.Control{}
			c.AckEvent.EventID = event.EventID
			control <- c

		case keyboard.OpStackUpdate:
			set := event.StackUpdate.Deployed
			control <- keyboard.Control{
				SetEnabled: &set,
			}
		}
	}

	for _, l := range listening {
		l.Cancel()
	}
}

// Listen blocks fetching logs from a container.
func Listen(ctx context.Context, control io.Writer, env cfg.Context, server runtime.Deployable, writerFactory func(ev runtime.ObserveEvent) io.Writer) error {
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
			var w io.Writer
			if writerFactory != nil {
				w = writerFactory(ev)
			} else {
				w = console.Output(ctx, humanReadable(ev))
			}
			ctx, cancel := context.WithCancel(ctx)

			if !newS.set(cancel, w) {
				// Raced with pod disappearing.
				return nil
			}

			return rt.Cluster().FetchLogsTo(ctx, ev.ContainerReference, runtime.FetchLogsOpts{
				TailLines: 30,
				Follow:    true,
			}, func(cll runtime.ContainerLogLine) {
				switch cll.Event {
				case runtime.ContainerLogLineEvent_LogLine:
					fmt.Fprintf(w, "%s\n", cll.LogLine)

				case runtime.ContainerLogLineEvent_Resuming:
					fmt.Fprintf(control, ">>> (log tail disconnected) resuming logging...\n")
				}
			})
		})

		return false, nil
	})
}

func humanReadable(ev runtime.ObserveEvent) string {
	left := ev.Deployable.GetName()
	if ev.ContainerReference.Kind != runtimepb.ContainerKind_PRIMARY {
		left = fmt.Sprintf("%s:%s", left, ev.ContainerReference.HumanReference)
	}

	return fmt.Sprintf("%s (%s)", left, ev.Version)
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

// On first write, calls onStart.
type writerWithHeader struct {
	onStart func(w io.Writer)
	w       io.Writer
}

func (w *writerWithHeader) Write(p []byte) (int, error) {
	if w.onStart != nil {
		w.onStart(w.w)
		w.onStart = nil
	}
	return w.w.Write(p)
}

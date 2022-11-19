// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package deploy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/kr/text"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/renderwait"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/schema/orchestration"
	runtimepb "namespacelabs.dev/foundation/schema/runtime"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/execution"
	"namespacelabs.dev/foundation/std/tasks"
)

const (
	maxDeployWait      = 5 * time.Second
	diagnosisInterval  = 3 * time.Second
	tailLinesOnFailure = 10
)

func MaybeRenderBlock(env cfg.Context, cluster runtime.ClusterNamespace, render bool) execution.WaitHandler {
	return func(ctx context.Context) (chan *orchestration.Event, func(context.Context) error) {
		if !render {
			return observeContainers(ctx, env, cluster, nil), func(ctx context.Context) error { return nil }
		}

		rwb := renderwait.NewBlock(ctx, "deploy")

		return observeContainers(ctx, env, cluster, rwb.Ch()), rwb.Wait
	}
}

func shouldCheckDiagnostics(commited, last time.Time, hasCrash bool, now time.Time) bool {
	if last.IsZero() {
		if hasCrash {
			return true
		}

		return now.Sub(commited) >= maxDeployWait
	}

	return now.Sub(last) >= diagnosisInterval
}

// observeContainers observes the deploy events (received from the returned channel) and updates the
// console through the `parent` channel.
func observeContainers(ctx context.Context, env cfg.Context, cluster runtime.ClusterNamespace, parent chan *orchestration.Event) chan *orchestration.Event {
	ch := make(chan *orchestration.Event)
	t := time.NewTicker(time.Second)

	type committedState struct {
		Commited      time.Time
		LastDiagnosis time.Time
		Help          string
		Waiting       []*runtimepb.ContainerWaitStatus
	}

	go func() {
		if parent != nil {
			defer close(parent)
		}

		defer t.Stop()

		commited := map[string]*committedState{} // Key is resource ID.

		checkRunDiagnosis := func() {
			for resourceID, state := range commited {
				all := []*runtimepb.ContainerUnitWaitStatus{}
				for _, w := range state.Waiting {
					all = append(all, w.Containers...)
					all = append(all, w.Initializers...)
				}

				if len(all) == 0 {
					// No containers are present yet, should still produce pod diagnostics.
					continue
				}

				hasCrash := false
				for _, ws := range all {
					if ws.Status.Crashed || ws.Status.Failed() {
						hasCrash = true
						break
					}
				}

				now := time.Now()
				if !shouldCheckDiagnostics(state.Commited, state.LastDiagnosis, hasCrash, now) {
					continue
				}

				state.LastDiagnosis = now

				buf := bytes.NewBuffer(nil)
				out := io.MultiWriter(buf, console.Debug(ctx))

				if state.Help != "" && !env.Environment().GetEphemeral() {
					fmt.Fprintf(out, "For more information, run:\n  %s\n", state.Help)
				}

				fmt.Fprintf(out, "Diagnostics retrieved at %s:\n", time.Now().Format("2006-01-02 15:04:05.000"))

				// XXX fetching diagnostics should not block forwarding events (above).
				for _, ws := range all {
					diagnostics, err := cluster.Cluster().FetchDiagnostics(ctx, ws.Reference)
					if err != nil {
						fmt.Fprintf(out, "Failed to retrieve diagnostics for %s: %v\n", ws.Reference.HumanReference, err)
						continue
					}

					fmt.Fprintf(out, "%s", ws.Reference.HumanReference)
					if diagnostics.RestartCount > 0 {
						fmt.Fprintf(out, " (restarted %d times)", diagnostics.RestartCount)
					}
					fmt.Fprintln(out, ":")

					w := runtime.WriteToWriter(text.NewIndentWriter(out, []byte("    ")))

					switch diagnostics.State {
					case runtimepb.Diagnostics_RUNNING:
						fmt.Fprintf(out, "  Running, logs (last %d lines):\n", tailLinesOnFailure)
						if err := cluster.Cluster().FetchLogsTo(ctx, ws.Reference, runtime.FetchLogsOpts{TailLines: tailLinesOnFailure}, w); err != nil {
							fmt.Fprintf(out, "Failed to retrieve logs for %s: %v\n", ws.Reference.HumanReference, err)
						}

					case runtimepb.Diagnostics_WAITING:
						fmt.Fprintf(out, "  Waiting: %s\n", diagnostics.WaitingReason)
						if diagnostics.Crashed {
							fmt.Fprintf(out, "  Crashed, logs (last %d lines):\n", tailLinesOnFailure)
							if err := cluster.Cluster().FetchLogsTo(ctx, ws.Reference, runtime.FetchLogsOpts{TailLines: tailLinesOnFailure, FetchLastFailure: true}, w); err != nil {
								fmt.Fprintf(out, "Failed to retrieve logs for %s: %v\n", ws.Reference.HumanReference, err)
							}
						}

					case runtimepb.Diagnostics_TERMINATED:
						if diagnostics.ExitCode > 0 {
							fmt.Fprintf(out, "  Failed: %s (exit code %d), logs (last %d lines):\n", diagnostics.TerminatedReason, diagnostics.ExitCode, tailLinesOnFailure)
							if err := cluster.Cluster().FetchLogsTo(ctx, ws.Reference, runtime.FetchLogsOpts{TailLines: tailLinesOnFailure, FetchLastFailure: true}, w); err != nil {
								fmt.Fprintf(out, "Failed to retrieve logs for %s: %v\n", ws.Reference.HumanReference, err)
							}
						}
					}
				}

				if parent != nil {
					parent <- &orchestration.Event{
						ResourceId:  resourceID,
						WaitDetails: buf.String(),
					}
				} else {
					diagnostics, err := cluster.FetchEnvironmentDiagnostics(ctx)
					if err != nil {
						fmt.Fprintf(out, "Failed to retrieve environment diagnostics: %v\n", err)
					} else {
						_ = tasks.Attachments(ctx).AttachSerializable("diagnostics.json", "", diagnostics)
					}
				}
			}
		}

		for {
			select {
			case <-ctx.Done():
				return

			case ev, ok := <-ch:
				if !ok {
					return
				}

				if parent != nil {
					parent <- ev
				}

				if ev.Stage >= orchestration.Event_COMMITTED {
					state, ok := commited[ev.ResourceId]
					if !ok {
						state = &committedState{
							Commited: ev.Timestamp.AsTime(),
						}

						if state.Commited.IsZero() {
							state.Commited = time.Now()
						}

						commited[ev.ResourceId] = state
					}

					if ev.Ready != orchestration.Event_READY {
						state.Waiting = nil
						state.Help = ev.RuntimeSpecificHelp

						failed := false
						for _, w := range ev.WaitStatus {
							if w.Opaque == nil {
								continue
							}

							cws := &runtimepb.ContainerWaitStatus{}
							if err := w.Opaque.UnmarshalTo(cws); err != nil {
								continue
							}
							state.Waiting = append(state.Waiting, cws)

							var ctrs []*runtimepb.ContainerUnitWaitStatus
							ctrs = append(ctrs, cws.Containers...)
							ctrs = append(ctrs, cws.Initializers...)

							for _, ctr := range ctrs {
								if ctr.Status.Crashed || ctr.Status.Failed() {
									failed = true
								}
							}
						}

						if failed {
							checkRunDiagnosis()
						}
					} else {
						delete(commited, ev.ResourceId)
					}
				}

			case <-t.C:
				checkRunDiagnosis()
			}
		}
	}()

	return ch
}

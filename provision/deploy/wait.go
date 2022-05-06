// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

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
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/runtime"
)

const (
	maxDeployWait      = 10 * time.Second
	tailLinesOnFailure = 10
)

func Wait(ctx context.Context, env ops.Environment, waiters []ops.Waiter) error {
	rwb := renderwait.NewBlock(ctx, "deploy")

	waitErr := ops.WaitMultiple(ctx, waiters, observeContainers(ctx, env, rwb.Ch()))

	// Make sure that rwb completes before further output below (for ordering purposes).
	if err := rwb.Wait(ctx); err != nil {
		if waitErr == nil {
			return err
		}
	}

	return waitErr
}

func observeContainers(ctx context.Context, env ops.Environment, parent chan ops.Event) chan ops.Event {
	ch := make(chan ops.Event)
	t := time.NewTicker(maxDeployWait)
	startedDiagnosis := true // After the first tick, we tick twice as fast.

	go func() {
		defer close(parent)
		defer t.Stop()

		// Keep track of the pending ContainerWaitStatus per resource type.
		pending := map[string][]runtime.ContainerWaitStatus{}
		helps := map[string]string{}

		runDiagnosis := func() {
			if startedDiagnosis {
				startedDiagnosis = false
				t.Reset(maxDeployWait / 2)
			}

			rt := runtime.For(ctx, env)
			for resourceID, wslist := range pending {
				all := []runtime.ContainerUnitWaitStatus{}
				for _, w := range wslist {
					all = append(all, w.Containers...)
					all = append(all, w.Initializers...)
				}
				if len(all) == 0 {
					// No containers are present yet, should still produce pod diagnostics.
					continue
				}

				buf := bytes.NewBuffer(nil)
				out := io.MultiWriter(buf, console.Debug(ctx))

				if help, ok := helps[resourceID]; ok && !env.Proto().GetEphemeral() {
					fmt.Fprintf(out, "For more information, run:\n  %s\n", help)
				}

				fmt.Fprintf(out, "Diagnostics retrieved at %s:\n", time.Now().Format("2006-01-02 15:04:05.000"))

				// XXX fetching diagnostics should not block forwarding events (above).
				for _, ws := range all {
					diagnostics, err := rt.FetchDiagnostics(ctx, ws.Reference)
					if err != nil {
						fmt.Fprintf(out, "Failed to retrieve diagnostics for %s: %v\n", ws.Reference.HumanReference(), err)
						continue
					}

					fmt.Fprintf(out, "%s", ws.Reference.HumanReference())
					if diagnostics.RestartCount > 0 {
						fmt.Fprintf(out, " (restarted %d times)", diagnostics.RestartCount)
					}
					fmt.Fprintln(out, ":")

					switch {
					case diagnostics.Running:
						fmt.Fprintf(out, "  Running, logs (last %d lines):\n", tailLinesOnFailure)
						if err := rt.FetchLogsTo(ctx, text.NewIndentWriter(out, []byte("    ")), ws.Reference, runtime.FetchLogsOpts{TailLines: tailLinesOnFailure}); err != nil {
							fmt.Fprintf(out, "Failed to retrieve logs for %s: %v\n", ws.Reference.HumanReference(), err)
						}

					case diagnostics.Waiting:
						fmt.Fprintf(out, "  Waiting: %s\n", diagnostics.WaitingReason)
						if diagnostics.Crashed {
							fmt.Fprintf(out, "  Crashed, logs (last %d lines):\n", tailLinesOnFailure)
							if err := rt.FetchLogsTo(ctx, text.NewIndentWriter(out, []byte("    ")), ws.Reference, runtime.FetchLogsOpts{TailLines: tailLinesOnFailure, FetchLastFailure: true}); err != nil {
								fmt.Fprintf(out, "Failed to retrieve logs for %s: %v\n", ws.Reference.HumanReference(), err)
							}
						}

					case diagnostics.Terminated:
						if diagnostics.ExitCode > 0 {
							fmt.Fprintf(out, "  Failed: %s (exit code %d), logs (last %d lines):\n", diagnostics.TerminatedReason, diagnostics.ExitCode, tailLinesOnFailure)
							if err := rt.FetchLogsTo(ctx, text.NewIndentWriter(out, []byte("  ")), ws.Reference, runtime.FetchLogsOpts{TailLines: tailLinesOnFailure, FetchLastFailure: true}); err != nil {
								fmt.Fprintf(out, "Failed to retrieve logs for %s: %v\n", ws.Reference.HumanReference(), err)
							}
						}
					}
				}

				parent <- ops.Event{
					ResourceID:  resourceID,
					WaitDetails: buf.String(),
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

				parent <- ev

				if ev.Ready != ops.Ready {
					pending[ev.ResourceID] = nil
					helps[ev.ResourceID] = ev.RuntimeSpecificHelp

					failed := false
					for _, w := range ev.WaitStatus {
						if cws, ok := w.(runtime.ContainerWaitStatus); ok {
							pending[ev.ResourceID] = append(pending[ev.ResourceID], cws)

							var ctrs []runtime.ContainerUnitWaitStatus
							ctrs = append(ctrs, cws.Containers...)
							ctrs = append(ctrs, cws.Initializers...)

							for _, ctr := range ctrs {
								if ctr.Status.Crashed || ctr.Status.Failed() {
									failed = true
								}
							}
						}
					}

					if failed && !startedDiagnosis {
						runDiagnosis()
					}
				} else {
					delete(pending, ev.ResourceID)
				}

			case <-t.C:
				runDiagnosis()
			}
		}
	}()

	return ch
}

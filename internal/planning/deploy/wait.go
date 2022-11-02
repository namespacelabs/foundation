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
	maxDeployWait      = 10 * time.Second
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

// observeContainers observes the deploy events (received from the returned channel) and updates the
// console through the `parent` channel.
func observeContainers(ctx context.Context, env cfg.Context, cluster runtime.ClusterNamespace, parent chan *orchestration.Event) chan *orchestration.Event {
	ch := make(chan *orchestration.Event)
	t := time.NewTicker(maxDeployWait)
	startedDiagnosis := true // After the first tick, we tick twice as fast.

	go func() {
		if parent != nil {
			defer close(parent)
		}

		defer t.Stop()

		// Keep track of the pending ContainerWaitStatus per resource type.
		pending := map[string][]*runtimepb.ContainerWaitStatus{}
		waitErrors := map[string][]string{}
		helps := map[string]string{}

		runDiagnosis := func() {
			if startedDiagnosis {
				startedDiagnosis = false
				t.Reset(maxDeployWait / 2)
			}

			for resourceID, wslist := range pending {
				all := []*runtimepb.ContainerUnitWaitStatus{}
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

				if errs, ok := waitErrors[resourceID]; ok {
					for _, msg := range errs {
						fmt.Fprintf(out, "%s\n", msg)
					}
				}

				if help, ok := helps[resourceID]; ok && !env.Environment().GetEphemeral() {
					fmt.Fprintf(out, "For more information, run:\n  %s\n", help)
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

					switch {
					case diagnostics.Running:
						fmt.Fprintf(out, "  Running, logs (last %d lines):\n", tailLinesOnFailure)
						if err := cluster.Cluster().FetchLogsTo(ctx, text.NewIndentWriter(out, []byte("    ")), ws.Reference, runtime.FetchLogsOpts{TailLines: tailLinesOnFailure}); err != nil {
							fmt.Fprintf(out, "Failed to retrieve logs for %s: %v\n", ws.Reference.HumanReference, err)
						}

					case diagnostics.Waiting:
						fmt.Fprintf(out, "  Waiting: %s\n", diagnostics.WaitingReason)
						if diagnostics.Crashed {
							fmt.Fprintf(out, "  Crashed, logs (last %d lines):\n", tailLinesOnFailure)
							if err := cluster.Cluster().FetchLogsTo(ctx, text.NewIndentWriter(out, []byte("    ")), ws.Reference, runtime.FetchLogsOpts{TailLines: tailLinesOnFailure, FetchLastFailure: true}); err != nil {
								fmt.Fprintf(out, "Failed to retrieve logs for %s: %v\n", ws.Reference.HumanReference, err)
							}
						}

					case diagnostics.Terminated:
						if diagnostics.ExitCode > 0 {
							fmt.Fprintf(out, "  Failed: %s (exit code %d), logs (last %d lines):\n", diagnostics.TerminatedReason, diagnostics.ExitCode, tailLinesOnFailure)
							if err := cluster.Cluster().FetchLogsTo(ctx, text.NewIndentWriter(out, []byte("  ")), ws.Reference, runtime.FetchLogsOpts{TailLines: tailLinesOnFailure, FetchLastFailure: true}); err != nil {
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

				if ev.Ready != orchestration.Event_READY {
					pending[ev.ResourceId] = nil
					waitErrors[ev.ResourceId] = nil
					helps[ev.ResourceId] = ev.RuntimeSpecificHelp

					failed := false
					for _, w := range ev.WaitStatus {
						if w.ErrorMessage != "" {
							waitErrors[ev.ResourceId] = append(waitErrors[ev.ResourceId], w.ErrorMessage)
						}

						if w.Opaque == nil {
							continue
						}

						cws := &runtimepb.ContainerWaitStatus{}
						if err := w.Opaque.UnmarshalTo(cws); err == nil {
							pending[ev.ResourceId] = append(pending[ev.ResourceId], cws)

							var ctrs []*runtimepb.ContainerUnitWaitStatus
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
					delete(pending, ev.ResourceId)
				}

			case <-t.C:
				runDiagnosis()
			}
		}
	}()

	return ch
}

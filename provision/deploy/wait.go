// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package deploy

import (
	"context"
	"fmt"
	"time"

	"github.com/kr/text"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/renderwait"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/tasks"
)

const maxDeployWait = 10 * time.Second

func Wait(ctx context.Context, env ops.Environment, servers []*schema.Server, waiters []ops.Waiter) error {
	rwb := renderwait.NewBlock(ctx, "deploy")
	err := ops.WaitMultiple(ctx, waiters, observeContainers(ctx, env, rwb.Ch()))

	// Make sure that rwb completes before further output below (for ordering purposes).
	if waitErr := rwb.Wait(ctx); waitErr != nil {
		if err == nil {
			err = waitErr
		}
	}

	return err
}

func observeContainers(ctx context.Context, env ops.Environment, parent chan ops.Event) chan ops.Event {
	ch := make(chan ops.Event)
	t := time.NewTimer(maxDeployWait)

	go func() {
		defer close(parent)
		defer t.Stop()

		// Keep track of the pending ContainerWaitStatus per resource type.
		m := map[string][]runtime.ContainerWaitStatus{}

		for {
			select {
			case <-ctx.Done():
				return

			case ev, ok := <-ch:
				if !ok {
					return
				}

				parent <- ev

				m[ev.ResourceID] = nil

				for _, w := range ev.WaitStatus {
					if cws, ok := w.(runtime.ContainerWaitStatus); ok {
						m[ev.ResourceID] = append(m[ev.ResourceID], cws)
					}
				}

			case <-t.C:
				out := console.TypedOutput(ctx, "deploy", tasks.CatOutputUs)
				fmt.Fprintf(out, "Deploying is taking too long, fetching diagnostics of the pending containers:\n")

				rt := runtime.For(env)
				for _, wslist := range m {
					for _, w := range wslist {
						all := []runtime.ContainerUnitWaitStatus{}
						all = append(all, w.Containers...)
						all = append(all, w.Initializers...)
						for _, ws := range all {
							diagnostics, err := rt.FetchDiagnostics(ctx, ws.Reference)
							if err != nil {
								fmt.Fprintf(out, "Failed to retrieve diagnostics for %s: %v\n", ws.Reference.HumanReference(), err)
								continue
							}

							wout := console.TypedOutput(ctx, ws.Reference.HumanReference(), tasks.CatOutputTool)

							switch {
							case diagnostics.Running:
								fmt.Fprintf(wout, "Log tail:\n")
								if err := rt.FetchLogsTo(ctx, text.NewIndentWriter(wout, []byte("  ")), ws.Reference, runtime.FetchLogsOpts{TailLines: 20}); err != nil {
									fmt.Fprintf(out, "Failed to retrieve logs for %s: %v\n", ws.Reference.HumanReference(), err)
								}

							case diagnostics.Waiting:
								fmt.Fprintf(wout, "Waiting: %s\n", diagnostics.WaitingReason)

							case diagnostics.Terminated:
								fmt.Fprintf(wout, "Failed: %s (exit code %d), last log tail:\n", diagnostics.TerminatedReason, diagnostics.ExitCode)
								if err := rt.FetchLogsTo(ctx, text.NewIndentWriter(wout, []byte("  ")), ws.Reference, runtime.FetchLogsOpts{TailLines: 20, FetchLastFailure: true}); err != nil {
									fmt.Fprintf(out, "Failed to retrieve logs for %s: %v\n", ws.Reference.HumanReference(), err)
								}
							}
						}
					}
				}
			}
		}
	}()

	return ch
}

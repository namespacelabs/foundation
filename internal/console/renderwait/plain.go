// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package renderwait

import (
	"context"
	"fmt"

	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/schema/orchestration"
)

type logRenderer struct {
	ch   chan *orchestration.Event
	done chan struct{}
}

func (rwb logRenderer) Ch() chan *orchestration.Event { return rwb.ch }
func (rwb logRenderer) Wait(ctx context.Context) error {
	select {
	case <-rwb.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (rwb logRenderer) Loop(ctx context.Context) {
	defer close(rwb.done) // Signal parent we're done.

	l := console.Output(ctx, "rwb")

	lastStage := map[string]orchestration.Event_Stage{}

	for {
		select {
		case <-ctx.Done():
			return

		case ev, ok := <-rwb.ch:
			if !ok {
				return
			}

			prevStage := lastStage[ev.ResourceId]
			lastStage[ev.ResourceId] = ev.Stage

			if ev.Stage == prevStage && ev.Stage != orchestration.Event_DONE {
				continue
			}

			switch ev.Stage {
			case orchestration.Event_WAITING:
				fmt.Fprintf(l, "waiting for %s\n", title(ev))
			case orchestration.Event_COMMITTED:
				fmt.Fprintf(l, "committed %s\n", title(ev))
			case orchestration.Event_RUNNING:
				fmt.Fprintf(l, "running %s\n", title(ev))
			case orchestration.Event_DONE:
				if ev.AlreadyExisted {
					fmt.Fprintf(l, "ready %s (no update required)\n", title(ev))
				} else {
					fmt.Fprintf(l, "ready %s\n", title(ev))
				}
			}
		}
	}
}

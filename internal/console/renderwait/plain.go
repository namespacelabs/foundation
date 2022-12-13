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

	for {
		select {
		case <-ctx.Done():
			return

		case ev, ok := <-rwb.ch:
			if !ok {
				return
			}

			fmt.Fprintf(l, "waiting (stage=%v alreadyExisted=%v) for id %s category %s title %s impl %v\n",
				ev.Stage, ev.AlreadyExisted, ev.ResourceId, ev.Category, title(ev), string(ev.ImplMetadata))
		}
	}
}

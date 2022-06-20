// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package renderwait

import (
	"context"
	"fmt"

	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/engine/ops"
)

type logRenderer struct {
	ch   chan ops.Event
	done chan struct{}
}

func (rwb logRenderer) Ch() chan ops.Event { return rwb.ch }
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

			if ev.Ready == ops.Unknown {
				continue
			}

			fmt.Fprintf(l, "waiting (ready=%v alreadyExisted=%v) for id %s category %s scope %s impl %v\n",
				ev.Ready == ops.Ready, ev.AlreadyExisted, ev.ResourceID, ev.Category, ev.Scope, ev.ImplMetadata)
		}
	}
}

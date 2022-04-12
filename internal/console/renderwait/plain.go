// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package renderwait

import (
	"context"

	"github.com/rs/zerolog"
	"namespacelabs.dev/foundation/internal/engine/ops"
)

type logRenderer struct {
	ch     chan ops.Event
	done   chan struct{}
	logger *zerolog.Logger
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

	for {
		select {
		case <-ctx.Done():
			return

		case ev, ok := <-rwb.ch:
			if !ok || ev.AllDone {
				return
			}

			if ev.Ready == ops.Unknown {
				continue
			}

			l := rwb.logger.Info().Str("id", ev.ResourceID).
				Str("category", ev.Category).
				Interface("scope", ev.Scope).
				Interface("impl", ev.ImplMetadata).
				Bool("ready", ev.Ready == ops.Ready)
			if ev.AlreadyExisted {
				l = l.Bool("alreadyExisted", ev.AlreadyExisted)
			}
			l.Msg("waiting")
		}
	}
}

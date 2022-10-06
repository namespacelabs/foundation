// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package execution

import (
	"context"

	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/schema/orchestration"
)

// A waiter implementation is required to close the received channel when it's done.
type Waiter func(context.Context, chan *orchestration.Event) error

func WaitMultipleWithHandler(ctx context.Context, waiters []Waiter, channelHandler WaitHandler) error {
	var ch chan *orchestration.Event
	var handleErr func(context.Context, error) error

	if channelHandler != nil {
		ch, handleErr = channelHandler(ctx)
	}

	waitErr := waitMultiple(ctx, waiters, ch)

	if handleErr != nil {
		return handleErr(ctx, waitErr)
	}

	return waitErr
}

// waitMultiple waits for multiple Waiters to become ready. If `ch` is not null,
// it receives state change events, emitted by the waiters themselves.
func waitMultiple(ctx context.Context, waiters []Waiter, ch chan *orchestration.Event) error {
	if len(waiters) == 1 {
		// Defer channel management to the child waiter as well.
		return waiters[0](ctx, ch)
	}

	if ch != nil {
		defer close(ch)
	}

	if len(waiters) == 0 {
		return nil
	}

	eg := executor.New(ctx, "ops.wait-multiple")

	for _, w := range waiters {
		w := w // Close on w.

		eg.Go(func(ctx context.Context) error {
			var chch chan *orchestration.Event

			// WaitUntilReady is responsible for closing the channel after it's done writing,
			// so we can't simply forward the channel we got in.
			if ch != nil {
				chch = make(chan *orchestration.Event)

				// It's important to have this channel forwarding run in the same executor,
				// to guarantee it doesn't return (and thus closes `ch`), before `chch` itself
				// is closed.
				eg.Go(func(ctx context.Context) error {
					for ev := range chch {
						ch <- ev
					}
					return nil
				})
			}

			return w(ctx, chch)
		})
	}

	return eg.Wait()
}

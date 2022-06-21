// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package ops

import (
	"context"

	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/schema"
)

type ReadyTriState string

const (
	Unknown  ReadyTriState = ""
	NotReady ReadyTriState = "not-ready"
	Ready    ReadyTriState = "ready"
)

type Event struct {
	ResourceID     string // Opaque value that uniquely identifies the resource.
	Kind           string // A resource identifier that is implementation specified.
	Category       string // A human-readable label that describes the resource.
	Scope          schema.PackageName
	Ready          ReadyTriState // `ready` after the resource is ready.
	AlreadyExisted bool
	ImplMetadata   interface{} // JSON serializable implementation-specific metadata.

	WaitStatus  []WaitStatus
	WaitDetails string
	// XXX move to a runtime/ specific type.
	RuntimeSpecificHelp string // Something like `kubectl -n foobar describe pod quux`
}

type WaitStatus interface {
	WaitStatus() string
}

// A waiter implementation is required to close the received channel when it's done.
type Waiter func(context.Context, chan Event) error

// WaitMultiple waits for multiple Waiters to become ready. If `ch` is not null,
// it receives state change events, emitted by the waiters themselves.
func WaitMultiple(ctx context.Context, waiters []Waiter, ch chan Event) error {
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

	eg, wait := executor.New(ctx, "ops.wait-multiple")

	for _, w := range waiters {
		w := w // Close on w.

		eg.Go(func(ctx context.Context) error {
			var chch chan Event

			// WaitUntilReady is responsible for closing the channel after it's done writing,
			// so we can't simply forward the channel we got in.
			if ch != nil {
				chch = make(chan Event)

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

	return wait()
}

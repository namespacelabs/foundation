// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package compute

import (
	"context"
	"sync"
	"time"

	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/tasks"
)

type hasAction interface {
	Action() *tasks.ActionEvent
}

type Promise[V any] struct {
	actionID tasks.ActionID
	c        hasAction
	mu       sync.Mutex
	waiters  []chan atom[V] // We use channels, to allow for select{}ing for cancelation.
	resolved atom[V]
	done     bool
}

type Future[V any] struct {
	actionID tasks.ActionID
	ch       chan atom[V]
	atom     atom[V]
}

type Result[V any] struct {
	Digest           schema.Digest
	NonDeterministic bool
	Value            V
}

type ResultWithTimestamp[V any] struct {
	Result[V]
	Set       bool
	Cached    bool
	ActionID  tasks.ActionID
	Started   time.Time
	Completed time.Time // When this value was computed (if known).

	revision uint64 // Used in a Continuous() flow.
}

type atom[V any] struct {
	value ResultWithTimestamp[V]
	err   error
}

func initializePromise[V any](p *Promise[V], c hasAction, id string) *Promise[V] {
	p.actionID = tasks.ActionID(id)
	p.c = c
	return p
}

func makePromise[V any](c hasAction, id string) *Promise[V] {
	return initializePromise(&Promise[V]{}, c, id)
}

func NewPromise[V any](g *Orch, action *tasks.ActionEvent, callback func(context.Context) (ResultWithTimestamp[V], error)) *Promise[V] {
	id := tasks.NewActionID()
	action = action.ID(id)
	p := makePromise[V](wrapHasAction{action}, id.String())

	g.Detach(action, func(ctx context.Context) error {
		result, err := callback(ctx)
		_ = p.resolve(result, err)
		return nil
	})

	return p
}

type wrapHasAction struct{ action *tasks.ActionEvent }

func (w wrapHasAction) Action() *tasks.ActionEvent { return w.action }

func (f *Promise[V]) resolve(v ResultWithTimestamp[V], err error) error {
	f.mu.Lock()
	resolved := atom[V]{v, err}
	f.resolved = resolved
	f.done = true
	waiters := f.waiters
	f.waiters = nil
	f.mu.Unlock()

	for _, w := range waiters {
		w <- resolved
		close(w)
	}

	return err
}

func (f *Promise[V]) fail(err error) error {
	return f.resolve(ResultWithTimestamp[V]{}, err)
}

func (f *Promise[V]) Future() *Future[V] {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.done {
		return &Future[V]{actionID: f.actionID, atom: f.resolved}
	}
	ch := make(chan atom[V], 1)
	f.waiters = append(f.waiters, ch)
	return &Future[V]{actionID: f.actionID, ch: ch}
}

func (r Result[V]) HasDigest() bool { return r.Digest.IsSet() }

func (f *Future[V]) Wait(ctx context.Context) (ResultWithTimestamp[V], error) {
	if f.ch != nil {
		if err := tasks.Action("compute.wait").Anchor(f.actionID).Run(ctx, func(ctx context.Context) error {
			select {
			case <-ctx.Done():
				return ctx.Err()

			case resolved, ok := <-f.ch:
				f.ch = nil

				if !ok {
					f.atom = atom[V]{err: context.Canceled}
				} else {
					f.atom = resolved
				}
			}

			return nil
		}); err != nil {
			return ResultWithTimestamp[V]{}, err
		}
	}

	return f.atom.value, f.atom.err
}

func valueFuture[V any](r ResultWithTimestamp[V]) *Promise[V] {
	return &Promise[V]{done: true, resolved: atom[V]{value: r}}
}

func ErrPromise[V any](err error) *Promise[V] {
	return &Promise[V]{done: true, resolved: atom[V]{err: err}}
}

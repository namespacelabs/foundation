// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package devworkflow

import (
	"context"
	"sync"

	"go.uber.org/atomic"
	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/observers"
	"namespacelabs.dev/foundation/internal/protos"
)

type opType int

const (
	pOpAddCh    opType = 1
	pOpRemoveCh opType = 2
	pOpNewData  opType = 3
)

type obsMsg struct {
	op         opType
	observer   *Observer
	message    *Update
	callbackCh chan struct{} // If set, will be closed after this message is handled.
}

type Observers struct {
	ch     chan obsMsg
	mu     sync.Mutex
	closed bool
}

type Observer struct {
	parent       *Observers
	allUpdates   chan *Update                     // Either this channel is set, or stackUpdates is.
	stackUpdates chan *observers.StackUpdateEvent // If only stackUpdates is set, observer only receives updates to the stack.
	closed       atomic.Bool
}

func NewObservers(ctx context.Context) *Observers {
	ch := make(chan obsMsg)
	go doLoop(ctx, ch)
	return &Observers{ch: ch}
}

func (obs *Observers) New(update *Update, stackUpdates bool) (*Observer, error) {
	cli := &Observer{parent: obs}
	if !stackUpdates {
		cli.allUpdates = make(chan *Update, 1)
	} else {
		cli.stackUpdates = make(chan *observers.StackUpdateEvent, 1)
	}

	if update != nil {
		// Write before anyone has the chance of closing the channel.
		cli.post(update)
	}

	if !obs.pushCheckClosed(obsMsg{op: pOpAddCh, observer: cli}) {
		return nil, fnerrors.New("was closed")
	}

	return cli, nil
}

func (obs *Observers) Publish(data *Update) {
	copy := protos.Clone(data)
	obs.pushCheckClosed(obsMsg{op: pOpNewData, message: copy})
}

func (obs *Observers) pushCheckClosed(op obsMsg) bool {
	obs.mu.Lock()
	defer obs.mu.Unlock()

	if obs.closed {
		return false
	}

	// This is a bit tricky as we keep the lock held while waiting for the
	// goroutine to consume our write. That means that a concurrent Close() will
	// also have to wait.
	obs.ch <- op
	return true
}

func (obs *Observers) Close() {
	obs.mu.Lock()
	defer obs.mu.Unlock()
	if obs.closed {
		return
	}

	obs.closed = true
	close(obs.ch)
}

func doLoop(ctx context.Context, ch chan obsMsg) {
	var observers []*Observer

	for op := range ch {
		switch op.op {
		case pOpAddCh:
			observers = append(observers, op.observer)
		case pOpRemoveCh:
			index := slices.Index(observers, op.observer)
			if index >= 0 {
				if op.observer.allUpdates != nil {
					close(op.observer.allUpdates)
				}
				if op.observer.stackUpdates != nil {
					close(op.observer.stackUpdates)
				}

				observers = slices.Delete(observers, index, index+1)
			}
		case pOpNewData:
			for _, obs := range observers {
				obs.post(op.message)
			}
		}

		if op.callbackCh != nil {
			close(op.callbackCh)
		}
	}

	// Make sure that any observers that were not canceled, become canceled.
	for _, obs := range observers {
		obs.closed.Store(true)
		if obs.allUpdates != nil {
			close(obs.allUpdates)
		}
		if obs.stackUpdates != nil {
			close(obs.stackUpdates)
		}
	}
}

func (o *Observer) Close() {
	// Only send close message once.
	if o.closed.CAS(false, true) {
		callback := make(chan struct{})
		if o.parent.pushCheckClosed(obsMsg{op: pOpRemoveCh, observer: o, callbackCh: callback}) {
			// Block until we're actually removed.
			<-callback
		}
	}
}

func (o *Observer) Events() chan *Update {
	return o.allUpdates
}

func (o *Observer) StackEvents() chan *observers.StackUpdateEvent {
	return o.stackUpdates
}

func (o *Observer) post(update *Update) {
	if o.stackUpdates != nil {
		if update.StackUpdate != nil {
			o.stackUpdates <- &observers.StackUpdateEvent{
				Env:   update.StackUpdate.Env,
				Stack: update.StackUpdate.Stack,
				Focus: update.StackUpdate.Focus,
			}
		}
	} else {
		o.allUpdates <- update
	}
}

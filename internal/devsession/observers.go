// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package devsession

import (
	"sync"

	"golang.org/x/exp/maps"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/observers"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/go-ids"
)

type Observers struct {
	mu        sync.Mutex
	closed    bool
	observers map[string]*Observer
}

type Observer struct {
	id           string
	parent       *Observers
	mu           sync.Mutex // Synchronizes Close() and post()
	closed       bool
	allUpdates   chan *Update                     // Either this channel is set, or stackUpdates is.
	stackUpdates chan *observers.StackUpdateEvent // If only stackUpdates is set, observer only receives updates to the stack.
}

func NewObservers() *Observers {
	return &Observers{observers: map[string]*Observer{}}
}

func (obs *Observers) New(initial *Update, stackUpdates bool) (*Observer, error) {
	obs.mu.Lock()
	defer obs.mu.Unlock()

	if obs.closed {
		return nil, fnerrors.InternalError("observers already closed")
	}

	cli := &Observer{id: ids.NewRandomBase32ID(8), parent: obs}
	if !stackUpdates {
		cli.allUpdates = make(chan *Update, 1)
	} else {
		cli.stackUpdates = make(chan *observers.StackUpdateEvent, 1)
	}

	if initial != nil {
		// Write before anyone has the chance of closing the channel.
		cli.post(initial)
	}

	// The map is cloned so that `Publish` can hold on to an instance, with the
	// certainty that it won't be concurrently modified.
	newObs := maps.Clone(obs.observers)
	newObs[cli.id] = cli
	obs.observers = newObs
	return cli, nil
}

func (obs *Observers) Publish(data *Update) {
	copy := protos.Clone(data)

	obs.mu.Lock()
	observers := obs.observers
	obs.mu.Unlock()

	for _, obs := range observers {
		obs.post(copy)
	}
}

func (obs *Observers) remove(o *Observer) {
	obs.mu.Lock()
	defer obs.mu.Unlock()

	if obs.closed {
		return
	}

	newObs := maps.Clone(obs.observers)
	delete(newObs, o.id)
	obs.observers = newObs
}

func (obs *Observers) Close() {
	obs.mu.Lock()
	defer obs.mu.Unlock()

	if !obs.closed {
		for _, obs := range obs.observers {
			obs.cleanup()
		}
		obs.observers = nil
	}

	obs.closed = true
}

func (o *Observer) Close() {
	o.parent.remove(o)
	o.cleanup()
}

func (o *Observer) cleanup() {
	o.mu.Lock()
	defer o.mu.Unlock()

	if !o.closed {
		o.closed = true
		if o.stackUpdates != nil {
			close(o.stackUpdates)
		} else {
			close(o.allUpdates)
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
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.closed {
		return
	}

	if o.stackUpdates != nil {
		if update.StackUpdate != nil {
			o.stackUpdates <- &observers.StackUpdateEvent{
				Env:         update.StackUpdate.Env,
				Stack:       update.StackUpdate.Stack,
				Focus:       update.StackUpdate.Focus,
				NetworkPlan: update.StackUpdate.NetworkPlan,
			}
		}
	} else {
		o.allUpdates <- update
	}
}

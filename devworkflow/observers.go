// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package devworkflow

import (
	"sync"

	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/internal/observers"
	"namespacelabs.dev/foundation/internal/protos"
)

type Observers struct {
	mu        sync.Mutex
	closed    bool
	observers []*Observer
}

type Observer struct {
	parent       *Observers
	allUpdates   chan *Update                     // Either this channel is set, or stackUpdates is.
	stackUpdates chan *observers.StackUpdateEvent // If only stackUpdates is set, observer only receives updates to the stack.
}

func NewObservers() *Observers {
	return &Observers{}
}

func (obs *Observers) New(update *Update, stackUpdates bool) *Observer {
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

	obs.mu.Lock()
	defer obs.mu.Unlock()

	obs.observers = append(slices.Clone(obs.observers), cli)

	return cli
}

func (obs *Observers) Publish(data *Update) {
	copy := protos.Clone(data)

	obs.mu.Lock()
	observers := obs.observers
	if obs.closed {
		observers = nil
	}
	obs.mu.Unlock()

	for _, obs := range observers {
		obs.post(copy)
	}
}

func (obs *Observers) remove(o *Observer) {
	obs.mu.Lock()
	defer obs.mu.Unlock()

	i := slices.Index(obs.observers, o)
	if i >= 0 {
		obs.observers = slices.Delete(obs.observers, i, i+1)
	}
}

func (obs *Observers) Close() {
	obs.mu.Lock()
	defer obs.mu.Unlock()
	obs.closed = true
}

func (o *Observer) Close() {
	o.parent.remove(o)
	if o.stackUpdates != nil {
		close(o.stackUpdates)
	} else {
		close(o.allUpdates)
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

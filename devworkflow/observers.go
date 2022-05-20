// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package devworkflow

import (
	"context"

	"go.uber.org/atomic"
	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/proto"
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
	closed atomic.Bool
}

type Observer struct {
	parent *Observers
	ch     chan *Update
	closed atomic.Bool
}

func NewObservers(ctx context.Context) *Observers {
	ch := make(chan obsMsg)
	go doLoop(ctx, ch)
	return &Observers{ch: ch}
}

func (obs *Observers) New(update *Update) *Observer {
	cli := &Observer{parent: obs, ch: make(chan *Update, 1)}
	if update != nil {
		// Write before anyone has the chance of closing the channel.
		cli.ch <- update
	}
	obs.push(obsMsg{op: pOpAddCh, observer: cli})
	return cli
}

func (obs *Observers) Publish(data *Update) {
	copy := proto.Clone(data).(*Update)
	obs.push(obsMsg{op: pOpNewData, message: copy})
}

func (obs *Observers) push(op obsMsg) bool {
	if !obs.closed.Load() {
		obs.ch <- op
		return true
	}

	return false
}

func (obs *Observers) Close() {
	if obs.closed.CAS(false, true) {
		close(obs.ch)
	}
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
				close(op.observer.ch)
				observers = slices.Delete(observers, index, index+1)
			}
		case pOpNewData:
			for _, obs := range observers {
				obs.ch <- op.message
			}
		}

		if op.callbackCh != nil {
			close(op.callbackCh)
		}
	}

	// Make sure that any observers that were not canceled, become canceled.
	for _, obs := range observers {
		obs.closed.Store(true)
		close(obs.ch)
	}
}

func (o *Observer) Close() {
	// Only send close message once.
	if o.closed.CAS(false, true) {
		callback := make(chan struct{})
		if o.parent.push(obsMsg{op: pOpRemoveCh, observer: o, callbackCh: callback}) {
			// Block until we're actually removed.
			<-callback
		}
	}
}

func (o *Observer) Events() chan *Update {
	return o.ch
}

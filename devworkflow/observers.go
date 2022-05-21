// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package devworkflow

import (
	"context"
	"sync"

	"go.uber.org/atomic"
	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/fnerrors"
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
	parent *Observers
	ch     chan *Update
	closed atomic.Bool
}

func NewObservers(ctx context.Context) *Observers {
	ch := make(chan obsMsg)
	go doLoop(ctx, ch)
	return &Observers{ch: ch}
}

func (obs *Observers) New(update *Update) (*Observer, error) {
	cli := &Observer{parent: obs, ch: make(chan *Update, 1)}
	if update != nil {
		// Write before anyone has the chance of closing the channel.
		cli.ch <- update
	}

	if !obs.pushCheckClosed(obsMsg{op: pOpAddCh, observer: cli}) {
		return nil, fnerrors.New("was closed")
	}

	return cli, nil
}

func (obs *Observers) Publish(data *Update) {
	copy := proto.Clone(data).(*Update)
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
		if o.parent.pushCheckClosed(obsMsg{op: pOpRemoveCh, observer: o, callbackCh: callback}) {
			// Block until we're actually removed.
			<-callback
		}
	}
}

func (o *Observer) Events() chan *Update {
	return o.ch
}

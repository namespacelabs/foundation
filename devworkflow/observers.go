// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package devworkflow

import (
	"context"

	"google.golang.org/protobuf/proto"
)

type opType int

const (
	pOpAddCh    opType = 1
	pOpRemoveCh opType = 2
	pOpNewData  opType = 3
)

type obsMsg struct {
	typ opType
	ch  chan *Update
	msg *Update
}

type Observers struct {
	done chan struct{}
	ch   chan obsMsg
}

func NewObservers(ctx context.Context) *Observers {
	ch := make(chan obsMsg)
	go doLoop(ctx, ch)
	return &Observers{ch: ch, done: make(chan struct{})}
}

func (obs *Observers) Add(ch chan *Update) {
	obs.push(obsMsg{typ: pOpAddCh, ch: ch})
}

func (obs *Observers) Remove(ch chan *Update) {
	obs.push(obsMsg{typ: pOpRemoveCh, ch: ch})
}

func (obs *Observers) Publish(data *Update) {
	copy := proto.Clone(data).(*Update)
	obs.push(obsMsg{typ: pOpNewData, msg: copy})
}

func (obs *Observers) push(op obsMsg) {
	select {
	case <-obs.done:
		// Channel closed
	case obs.ch <- op:
	}
}

func (obs *Observers) Close() {
	close(obs.done)
	close(obs.ch)
}

func doLoop(ctx context.Context, ch chan obsMsg) {
	var observers []chan *Update

	for op := range ch {
		switch op.typ {
		case pOpAddCh:
			observers = append(observers, op.ch)
		case pOpRemoveCh:
			var newObservers []chan *Update
			for _, obs := range observers {
				if obs != op.ch {
					newObservers = append(newObservers, obs)
				}
			}
			observers = newObservers
		case pOpNewData:
			for _, obs := range observers {
				obs <- op.msg
			}
		}
	}
}

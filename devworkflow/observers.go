// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package devworkflow

import (
	"context"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/workspace/tasks"
)

type opType int
type JSON []byte

const (
	pOpAddCh    opType = 1
	pOpRemoveCh opType = 2
	pOpNewData  opType = 3
)

type obOp struct {
	typ opType
	ch  chan JSON
	msg JSON
}

type Observers struct {
	done chan struct{}
	ch   chan obOp
}

func (obs *Observers) Add(ch chan JSON) {
	if !obs.isClosed() {
		obs.ch <- obOp{typ: pOpAddCh, ch: ch}
	}
}

func (obs *Observers) Remove(ch chan JSON) {
	if !obs.isClosed() {
		obs.ch <- obOp{typ: pOpRemoveCh, ch: ch}
	}
}

func (obs *Observers) MarshalAndPublish(pr tasks.ProtoResolver, msg proto.Message) error {
	data, err := tasks.TryProtoAsJson(pr, msg, false)
	if err != nil {
		return err
	}
	obs.Publish(data)
	return nil
}

func (obs *Observers) Publish(data JSON) {
	if !obs.isClosed() {
		obs.ch <- obOp{typ: pOpNewData, msg: data}
	}
}

func (obs *Observers) isClosed() bool {
	select {
	case <-obs.done:
		return true
	default:
		return false
	}
}

func (obs *Observers) Close() {
	if !obs.isClosed() {
		close(obs.done)
		close(obs.ch)
	}
}

func NewObservers(ctx context.Context) *Observers {
	ch := make(chan obOp)
	go doLoop(ctx, ch)
	return &Observers{ch: ch, done: make(chan struct{})}
}

func doLoop(ctx context.Context, ch chan obOp) {
	var observers []chan JSON

	for op := range ch {
		switch op.typ {
		case pOpAddCh:
			observers = append(observers, op.ch)
		case pOpRemoveCh:
			var newObservers []chan JSON
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
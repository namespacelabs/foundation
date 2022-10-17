// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tasks

import (
	"context"
	"io"
	"sync"

	"namespacelabs.dev/foundation/internal/console/common"
	"namespacelabs.dev/foundation/std/tasks/protocol"
)

// Keeps track of which actions are running, and have run in the past.
type statefulState struct {
	parent      ActionSink
	keepHistory bool

	mu        sync.Mutex
	running   []*RunningAction
	observers []Observer

	allTasks       []*protocol.Task
	protoIndex     map[ActionID]int
	allAttachments map[ActionID]*EventAttachments
}

type Observer interface {
	OnStart(*RunningAction)
	OnUpdate(*RunningAction)
	OnDone(*RunningAction)
}

type StatefulSink struct{ state *statefulState }

var _ ActionSink = &statefulState{}

func NewStatefulSink(parent ActionSink, keepHistory bool) *StatefulSink {
	return &StatefulSink{&statefulState{
		parent:         parent,
		protoIndex:     map[ActionID]int{},
		allAttachments: map[ActionID]*EventAttachments{},
		keepHistory:    keepHistory,
	}}
}

func WithStatefulSink(ctx context.Context) (context.Context, *StatefulSink) {
	state := NewStatefulSink(SinkFrom(ctx), true)
	return WithSink(ctx, state.Sink()), state
}

func (s *StatefulSink) Sink() ActionSink { return s.state }

func (s *StatefulSink) HistoricReaderByName(id ActionID, name string) io.ReadCloser {
	s.state.mu.Lock()
	defer s.state.mu.Unlock()

	if attachments, ok := s.state.allAttachments[id]; ok {
		return attachments.ReaderByName(name)
	}

	return nil
}

func (s *StatefulSink) History(max int, filter func(*protocol.Task) bool) []*protocol.Task {
	s.state.mu.Lock()
	defer s.state.mu.Unlock()

	var filtered []*protocol.Task
	for _, t := range s.state.allTasks {
		if filter == nil || filter(t) {
			filtered = append(filtered, t)
		}
	}

	start := 0
	if len(filtered) > max {
		start = len(filtered) - max
	}

	history := filtered[start:]
	return history
}

// Recursively returns the action and all of its callers (leaf action first).
func (s *StatefulSink) Trace(id ActionID) (trace []*protocol.Task) {
	// XXX: the implementation is rather inefficient.
	next := s.state.runningAction(id)
	for next != nil {
		trace = append(trace, next.Data.Proto())
		if next.Data.ParentID != "" {
			next = s.state.runningAction(next.Data.ParentID)
		} else if next.Data.AnchorID != "" {
			next = s.state.waitingAction(next.Data.AnchorID)
		} else {
			next = nil
		}
	}
	return
}

func (s *StatefulSink) Observe(obs Observer) func() {
	s.state.mu.Lock()
	s.state.observers = append(s.state.observers, obs)
	s.state.mu.Unlock()

	return func() {
		s.state.mu.Lock()
		defer s.state.mu.Unlock()
		for k, was := range s.state.observers {
			if was == obs {
				s.state.observers = append(s.state.observers[0:k], s.state.observers[k+1:]...)
				return
			}
		}
	}
}

func (s *statefulState) runningAction(id ActionID) *RunningAction {
	for _, a := range s.running {
		if a.ID() == id {
			return a
		}
	}
	return nil
}

func (s *statefulState) waitingAction(id ActionID) *RunningAction {
	for _, a := range s.running {
		if a.Data.AnchorID == id {
			return a
		}
	}
	return nil
}

func (s *statefulState) addToRunning(ra *RunningAction) []Observer {
	p := ra.Proto()

	s.mu.Lock()
	if _, ok := s.protoIndex[ra.Data.ActionID]; !ok {
		s.running = append(s.running, ra)

		if s.keepHistory {
			s.allAttachments[ra.Data.ActionID] = ra.attachments
			index := len(s.allTasks)
			s.allTasks = append(s.allTasks, p)
			s.protoIndex[ra.Data.ActionID] = index
		}
	}
	observers := s.observers
	s.mu.Unlock()

	return observers
}

func (s *statefulState) Waiting(ra *RunningAction) {
	if s.parent != nil {
		s.parent.Waiting(ra)
	}
	s.addToRunning(ra)
}

func (s *statefulState) Started(ra *RunningAction) {
	if s.parent != nil {
		s.parent.Started(ra)
	}

	observers := s.addToRunning(ra)
	for _, obs := range observers {
		obs.OnStart(ra)
	}
}

func (s *statefulState) removeFromRunning(ra *RunningAction) {
	p := ra.Proto()

	s.mu.Lock()
	observers := s.observers
	for k, running := range s.running {
		if running.ID() == ra.ID() {
			s.running = append(s.running[0:k], s.running[k+1:]...)
			break
		}
	}

	if index, ok := s.protoIndex[ra.Data.ActionID]; ok {
		s.allTasks[index] = p // Update with completed, etc.
	}

	s.mu.Unlock()

	for _, obs := range observers {
		obs.OnDone(ra)
	}
}

func (s *statefulState) Done(ra *RunningAction) {
	if s.parent != nil {
		s.parent.Done(ra)
	}
	s.removeFromRunning(ra)
}

func (s *statefulState) Instant(ev *EventData) {
	if s.parent != nil {
		s.parent.Instant(ev)
	}
}

func (s *statefulState) AttachmentsUpdated(actionID ActionID, data *ResultData) {
	if s.parent != nil {
		s.parent.AttachmentsUpdated(actionID, data)
	}

	s.mu.Lock()
	observers := s.observers
	var r *RunningAction
	for _, running := range s.running {
		if running.ID() == actionID {
			r = running
			break
		}
	}
	if r != nil {
		p := r.Proto()
		if index, ok := s.protoIndex[actionID]; ok {
			s.allTasks[index] = p
		}
	}
	s.mu.Unlock()

	if r != nil {
		for _, obs := range observers {
			obs.OnUpdate(r)
		}
	}
}

func (s *statefulState) Output(name, contentType string, outputType common.CatOutputType) io.Writer {
	return nil
}

func (s *statefulState) Unwrap() ActionSink {
	return s.parent
}

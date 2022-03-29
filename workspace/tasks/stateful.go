// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tasks

import (
	"context"
	"io"
	"sync"

	"namespacelabs.dev/foundation/workspace/tasks/protocol"
)

var ActionRetentionMaxCount = -1 // No limit.

// Keeps track of which actions are running, and have run in the past.
type statefulState struct {
	parent ActionSink

	mu        sync.Mutex
	running   []*RunningAction
	observers []Observer

	allProtos      []*protocol.Task
	protoIndex     map[string]int
	allAttachments map[string]*EventAttachments
}

type Observer interface {
	OnStart(*RunningAction)
	OnUpdate(*RunningAction)
	OnDone(*RunningAction)
}

type StatefulSink struct{ s *statefulState }

var _ ActionSink = &statefulState{}

func WithStatefulSink(ctx context.Context) (context.Context, *StatefulSink) {
	s := &statefulState{
		parent:         SinkFrom(ctx),
		protoIndex:     map[string]int{},
		allAttachments: map[string]*EventAttachments{},
	}

	return WithSink(ctx, s), &StatefulSink{s}
}

func (s *StatefulSink) HistoricReaderByName(id, name string) io.ReadCloser {
	s.s.mu.Lock()
	defer s.s.mu.Unlock()

	if attachments, ok := s.s.allAttachments[id]; ok {
		return attachments.ReaderByName(name)
	}

	return nil
}

func (s *StatefulSink) History(max int, filter func(*protocol.Task) bool) []*protocol.Task {
	s.s.mu.Lock()
	defer s.s.mu.Unlock()

	var filtered []*protocol.Task
	for _, t := range s.s.allProtos {
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

func (s *StatefulSink) Observe(obs Observer) func() {
	s.s.mu.Lock()
	s.s.observers = append(s.s.observers, obs)
	s.s.mu.Unlock()

	return func() {
		s.s.mu.Lock()
		defer s.s.mu.Unlock()
		for k, was := range s.s.observers {
			if was == obs {
				s.s.observers = append(s.s.observers[0:k], s.s.observers[k+1:]...)
				return
			}
		}
	}
}

func (s *statefulState) addToRunning(ra *RunningAction) []Observer {
	p := ra.Proto()

	s.mu.Lock()
	if _, ok := s.protoIndex[ra.data.actionID]; !ok {
		s.running = append(s.running, ra)
		s.allAttachments[ra.data.actionID] = ra.attachments
		index := len(s.allProtos)
		s.allProtos = append(s.allProtos, p)
		s.protoIndex[ra.data.actionID] = index
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

	if index, ok := s.protoIndex[ra.data.actionID]; ok {
		s.allProtos[index] = p // Update with completed, etc.
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

func (s *statefulState) AttachmentsUpdated(actionID string, data *resultData) {
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
			s.allProtos[index] = p
		}
	}
	s.mu.Unlock()

	if r != nil {
		for _, obs := range observers {
			obs.OnUpdate(r)
		}
	}
}
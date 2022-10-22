// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package service

import (
	"fmt"
	"sync"
	"time"
)

type leaser struct {
	mu     sync.Mutex
	cond   *sync.Cond
	active map[string]struct{}
	last   map[string]time.Time
}

func newLeaser() *leaser {
	l := &leaser{
		active: make(map[string]struct{}),
		last:   make(map[string]time.Time),
	}

	l.cond = sync.NewCond(&l.mu)
	return l
}

var errDeployPlanTooOld = fmt.Errorf("incoming deployment is too old")

func (l *leaser) acquireLease(id string, new time.Time) (func(), error) {
	if l == nil {
		return nil, nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	for {
		if last, ok := l.last[id]; ok && last.After(new) {
			return nil, errDeployPlanTooOld
		}

		if _, ok := l.active[id]; !ok {
			l.active[id] = struct{}{}
			l.last[id] = new

			return func() {
				l.mu.Lock()
				defer l.mu.Unlock()

				delete(l.active, id)
				l.cond.Broadcast()
			}, nil
		}

		l.cond.Wait()
	}

}

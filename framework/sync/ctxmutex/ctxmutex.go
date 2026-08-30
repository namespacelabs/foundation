// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package ctxmutex

import "context"

type Mutex struct {
	ch chan struct{}
}

func NewMutex() *Mutex {
	return &Mutex{make(chan struct{}, 1)}
}

func (mu *Mutex) Lock(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return false
	case mu.ch <- struct{}{}:
		return true
	}
}

func (mu *Mutex) MustLock() {
	mu.ch <- struct{}{}
}

func (mu *Mutex) Unlock() {
	<-mu.ch
}

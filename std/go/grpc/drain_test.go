// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package grpc

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestLameduckRegistrationConcurrentWithStart(t *testing.T) {
	lameduckMu.Lock()
	originalFuncs := LameduckFuncsByName
	originalStarted := lameduckStarted
	LameduckFuncsByName = map[string]func(){}
	lameduckStarted = false
	lameduckMu.Unlock()

	t.Cleanup(func() {
		lameduckMu.Lock()
		LameduckFuncsByName = originalFuncs
		lameduckStarted = originalStarted
		lameduckMu.Unlock()
	})

	var calls atomic.Int32
	start := make(chan struct{})
	var funcs map[string]func()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		SetNamedLameduckFunc("test", func() { calls.Add(1) })
	}()
	go func() {
		defer wg.Done()
		<-start
		funcs = BeginLameduck()
	}()

	close(start)
	wg.Wait()

	for _, f := range funcs {
		f()
	}

	if got := calls.Load(); got != 1 {
		t.Fatalf("expected lameduck function to run once, got %d", got)
	}
}

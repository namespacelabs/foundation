// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package servercore

import (
	"reflect"
	"sync"
	"testing"
)

// TestRunShutdownPhases verifies that all lameduck functions run before any
// drain function (so clients get the GOAWAY signal before drain phase blocks
// on in-flight work) and that DrainFunc runs before DrainFuncsByName.
func TestRunShutdownPhases(t *testing.T) {
	var (
		mu     sync.Mutex
		events []string
	)
	record := func(name string) func() {
		return func() {
			mu.Lock()
			events = append(events, name)
			mu.Unlock()
		}
	}

	lameducks := map[string]func(){
		"lame.a": record("lame.a"),
		"lame.b": record("lame.b"),
	}
	drainFuncs := map[string]func(){
		"drain.x": record("drain.x"),
		"drain.y": record("drain.y"),
	}
	drainFunc := record("drain-singleton")

	runShutdownPhases(lameducks, drainFunc, drainFuncs)

	// Map iteration order is non-deterministic, so we only assert the
	// phase boundaries: every "lame.*" event must come before every
	// "drain*" event, and "drain-singleton" must come before all
	// "drain.*" events.
	lameDone := map[string]bool{}
	drainSeen := false
	singletonAt := -1
	firstNamedDrainAt := -1
	for i, ev := range events {
		switch {
		case ev == "drain-singleton":
			singletonAt = i
			drainSeen = true
		case len(ev) >= 5 && ev[:5] == "lame.":
			if drainSeen {
				t.Errorf("lameduck %q ran after a drain func; events=%v", ev, events)
			}
			lameDone[ev] = true
		case len(ev) >= 6 && ev[:6] == "drain.":
			if !drainSeen {
				t.Errorf("drain func %q ran before drain-singleton; events=%v", ev, events)
			}
			if firstNamedDrainAt == -1 {
				firstNamedDrainAt = i
			}
			drainSeen = true
		}
	}

	if singletonAt == -1 {
		t.Errorf("drain-singleton never ran; events=%v", events)
	}
	if firstNamedDrainAt != -1 && singletonAt > firstNamedDrainAt {
		t.Errorf("drain-singleton ran after named drain func; events=%v", events)
	}

	wantLameducks := []string{"lame.a", "lame.b"}
	gotLameducks := make([]string, 0, len(lameDone))
	for k := range lameDone {
		gotLameducks = append(gotLameducks, k)
	}
	if !sameSet(gotLameducks, wantLameducks) {
		t.Errorf("expected lameducks %v to all run, got %v; events=%v", wantLameducks, gotLameducks, events)
	}
}

// TestRunShutdownPhases_NilDrainFunc verifies the function is robust to a
// nil top-level drain func.
func TestRunShutdownPhases_NilDrainFunc(t *testing.T) {
	called := []string{}
	runShutdownPhases(
		map[string]func(){"l": func() { called = append(called, "l") }},
		nil,
		map[string]func(){"d": func() { called = append(called, "d") }},
	)
	if !reflect.DeepEqual(called, []string{"l", "d"}) {
		t.Errorf("expected [l d], got %v", called)
	}
}

func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	m := map[string]int{}
	for _, s := range a {
		m[s]++
	}
	for _, s := range b {
		m[s]--
	}
	for _, c := range m {
		if c != 0 {
			return false
		}
	}
	return true
}

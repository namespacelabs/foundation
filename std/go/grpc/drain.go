// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package grpc

import (
	"maps"
	"sync"

	"namespacelabs.dev/foundation/std/go/core"
)

var (
	DrainFunc        func()
	DrainFuncsByName = map[string]func(){}

	lameduckMu      sync.Mutex
	lameduckStarted bool

	// LameduckFuncsByName are invoked after MarkShutdownStarted and before any
	// drain function. Their purpose is to signal to clients that the server is
	// going away (e.g. by sending HTTP/2 GOAWAY frames) without waiting for
	// in-flight work to drain. Lameduck functions should return quickly; any
	// long-running drain logic belongs in DrainFunc/DrainFuncsByName.
	LameduckFuncsByName = map[string]func(){}
)

func SetDrainFunc(f func()) {
	core.AssertNotRunning("grpc.SetDrainFunc")

	if DrainFunc != nil {
		panic("drain func was already set")
	}

	DrainFunc = f
}

func SetNamedDrainFunc(name string, f func()) {
	core.AssertNotRunning("grpc.SetDrainFunc")

	if _, ok := DrainFuncsByName[name]; ok {
		panic("drain func was already set")
	}

	DrainFuncsByName[name] = f
}

// SetNamedLameduckFunc registers a lameduck function that runs after
// MarkShutdownStarted and before any drain function. Lameduck functions
// signal to clients that the server is going away (typically by triggering
// HTTP/2 GOAWAY) and should return quickly.
//
// SetNamedLameduckFunc may be called both during initialization and during
// Listen (e.g. by the foundation framework itself once the http server has
// been constructed).
func SetNamedLameduckFunc(name string, f func()) {
	lameduckMu.Lock()
	if _, ok := LameduckFuncsByName[name]; ok {
		lameduckMu.Unlock()
		panic("lameduck func was already set")
	}

	LameduckFuncsByName[name] = f
	runNow := lameduckStarted
	lameduckMu.Unlock()

	// Listener handlers may finish initialization concurrently with signal
	// handling. If lameduck has already started, run newly registered hooks
	// immediately so the listener cannot miss the shutdown phase.
	if runNow {
		core.ZLog.Info().Str("name", name).Msg("running late lameduck func")
		f()
	}
}

// BeginLameduck marks the start of the lameduck phase and returns a stable
// snapshot of the registered functions. Functions registered after this call
// are invoked immediately by SetNamedLameduckFunc.
func BeginLameduck() map[string]func() {
	lameduckMu.Lock()
	defer lameduckMu.Unlock()

	if lameduckStarted {
		return nil
	}

	lameduckStarted = true
	return maps.Clone(LameduckFuncsByName)
}

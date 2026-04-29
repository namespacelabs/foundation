// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package grpc

import (
	"namespacelabs.dev/foundation/std/go/core"
)

var (
	DrainFunc        func()
	DrainFuncsByName = map[string]func(){}

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
	if _, ok := LameduckFuncsByName[name]; ok {
		panic("lameduck func was already set")
	}

	LameduckFuncsByName[name] = f
}

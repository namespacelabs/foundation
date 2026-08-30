// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package core

import (
	"go.uber.org/atomic"
)

var (
	running struct {
		is atomic.Bool
	}
)

func AssertNotRunning(what string) {
	if running.is.Load() {
		Log.Fatalf("tried to call %s after the server has been initialized", what)
	}
}

func InitializationDone() {
	running.is.Store(true)
}

// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package core

import sync "sync"

var (
	running struct {
		mu sync.Mutex
		is bool
	}
)

func AssertNotRunning(what string) {
	running.mu.Lock()
	isRunning := running.is
	running.mu.Unlock()
	if isRunning {
		Log.Fatalf("tried to call %s after the server has been initialized", what)
	}
}

func InitializationDone() {
	running.mu.Lock()
	running.is = true
	running.mu.Unlock()
}
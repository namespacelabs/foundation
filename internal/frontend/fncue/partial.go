// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fncue

import "cuelang.org/go/cue"

type CueV struct{ Val cue.Value }

// Represents a Cue value alongside a list of keys that are *left* to be resolved and filled later.
type Partial struct {
	CueV
	Ctx        *snapshotCache
	Left       []KeyAndPath
	Package    CuePackage
	CueImports []CuePackage
}

func SerializedEval[V any](p *Partial, f func() (V, error)) (V, error) {
	p.Ctx.mu.Lock()
	defer p.Ctx.mu.Unlock()

	return f()
}

func SerializedEval3[V any, T any](p *Partial, f func() (V, T, error)) (V, T, error) {
	p.Ctx.mu.Lock()
	defer p.Ctx.mu.Unlock()

	return f()
}

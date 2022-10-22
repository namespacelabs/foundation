// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package compute

import (
	"context"
	"fmt"

	"namespacelabs.dev/foundation/std/tasks"
)

func Collect[V any](ev *tasks.ActionEvent, computables ...Computable[V]) Computable[[]ResultWithTimestamp[V]] {
	return &collect[V]{ev: ev, computables: computables}
}

type collect[V any] struct {
	ev          *tasks.ActionEvent
	computables []Computable[V]

	LocalScoped[[]ResultWithTimestamp[V]]
}

func (c *collect[V]) computableName(k int) string {
	name, _ := tasks.NameOf(c.ev)
	return fmt.Sprintf("%s[%d]", name, k)
}

func (c *collect[V]) Inputs() *In {
	in := Inputs()
	for k, computable := range c.computables {
		if computable != nil {
			in = in.Computable(c.computableName(k), computable)
		}
	}
	return in
}

func (c *collect[V]) Action() *tasks.ActionEvent { return c.ev }

func (c *collect[V]) Compute(ctx context.Context, deps Resolved) ([]ResultWithTimestamp[V], error) {
	results := make([]ResultWithTimestamp[V], len(c.computables))

	for k, computable := range c.computables {
		if computable == nil {
			continue
		}

		v, _ := GetDep(deps, computable, c.computableName(k))
		var typed ResultWithTimestamp[V]
		typed.ActionID = v.ActionID
		typed.Started = v.Started
		typed.Completed = v.Completed
		typed.Set = true
		typed.Value = v.Value
		typed.Cached = v.Cached
		typed.Digest = v.Digest
		typed.NonDeterministic = v.NonDeterministic
		results[k] = typed
	}

	return results, nil
}

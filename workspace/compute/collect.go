// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package compute

import (
	"context"
	"fmt"

	"namespacelabs.dev/foundation/workspace/tasks"
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
		in = in.Computable(c.computableName(k), computable)
	}
	return in
}

func (c *collect[V]) Action() *tasks.ActionEvent { return c.ev }

func (c *collect[V]) Compute(ctx context.Context, deps Resolved) ([]ResultWithTimestamp[V], error) {
	var results []ResultWithTimestamp[V]

	for k, computable := range c.computables {
		v, _ := GetDep(deps, computable, c.computableName(k))
		var typed ResultWithTimestamp[V]
		typed.ActionID = v.ActionID
		typed.Started = v.Started
		typed.Completed = v.Completed
		typed.Value = v.Value
		typed.Cached = v.Cached
		typed.Digest = v.Digest
		typed.NonDeterministic = v.NonDeterministic
		results = append(results, typed)
	}

	return results, nil
}

// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package compute

import (
	"context"

	"namespacelabs.dev/foundation/workspace/tasks"
)

// Consume ensures that the specified method is called on the value when it is
// computed, and before other computables that depend on the return value of
// Consume.
func Consume[V any](action *tasks.ActionEvent, from Computable[V], compute func(context.Context, ResultWithTimestamp[V]) error) Computable[V] {
	return Map(action, Inputs().Computable("from", from), Output{
		NotCacheable: true, // There's no value in retaining these intermediary artifacts.
	}, func(ctx context.Context, r Resolved) (V, error) {
		v, ok := GetDep(r, from, "from")
		if !ok {
			panic("missing from")
		}
		err := compute(ctx, v)
		return v.Value, err
	})
}

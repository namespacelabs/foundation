// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package compute

import (
	"context"

	"namespacelabs.dev/foundation/framework/sync/ctxmutex"
	"namespacelabs.dev/foundation/std/tasks"
)

func WithLock[T any](ctx context.Context, key string, make func(context.Context) (T, error)) (T, error) {
	g := On(ctx)

	g.mu.Lock()
	ser, ok := g.serialization[key]
	if !ok {
		ser = ctxmutex.NewMutex()
		g.serialization[key] = ser
	}
	g.mu.Unlock()

	return tasks.Return(ctx, tasks.Action("serialized").Arg("key", key), func(ctx context.Context) (T, error) {
		if !ser.Lock(ctx) {
			var zero T
			return zero, ctx.Err()
		}

		defer ser.Unlock()

		return make(ctx)
	})
}

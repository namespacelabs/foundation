// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package ops

import (
	"context"

	"namespacelabs.dev/foundation/internal/fnerrors"
)

type injectionKey string

var _injectionKey injectionKey = "fn.ops.injection"

type Injection[V any] struct {
	key string
}

func Define[V any](key string) Injection[V] {
	return Injection[V]{key}
}

func (inj Injection[V]) With(ctx context.Context, value V) context.Context {
	state := ctx.Value(_injectionKey)

	var existing []injectionInstance
	if state != nil {
		existing = state.(injections).instances
	}

	return context.WithValue(ctx, _injectionKey, injections{
		instances: append([]injectionInstance{{inj.key, value}}, existing...),
	})
}

// Get can be invoked within a serialized invocation implementation.
func Get[V any](ctx context.Context, inj Injection[V]) (V, error) {
	var empty V

	state := ctx.Value(_injectionKey)
	if state == nil {
		return empty, fnerrors.InternalError("%s: no injection context in context", inj.key)
	}

	for _, instance := range state.(injections).instances {
		if instance.key == inj.key {
			return instance.value.(V), nil
		}
	}

	return empty, fnerrors.InternalError("%s: no such injected key", inj.key)
}

type injections struct {
	instances []injectionInstance
}

type injectionInstance struct {
	key   string
	value any
}

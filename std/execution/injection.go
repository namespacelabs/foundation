// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package execution

import (
	"context"

	"namespacelabs.dev/foundation/internal/fnerrors"
)

type injectionKey string

var _injectionKey injectionKey = "fn.execution.Injection"

type Injection[V any] struct {
	key string
}

func Define[V any](key string) Injection[V] {
	return Injection[V]{key}
}

func (inj Injection[V]) With(value V) InjectionInstance {
	return InjectionInstance{inj.key, value}
}

func injectValues(ctx context.Context, instances ...InjectionInstance) context.Context {
	if len(instances) == 0 {
		return ctx
	}

	state := ctx.Value(_injectionKey)

	var existing []InjectionInstance
	if state != nil {
		existing = state.(injections).instances
	}

	return context.WithValue(ctx, _injectionKey, injections{
		instances: append(instances, existing...),
	})
}

// Get can be invoked within a serialized invocation implementation.
func Get[V any](ctx context.Context, inj Injection[V]) (V, error) {
	var empty V

	state := ctx.Value(_injectionKey)
	if state == nil {
		return empty, fnerrors.InternalError("%q: no injection context in context", inj.key)
	}

	for _, instance := range state.(injections).instances {
		if instance.key == inj.key {
			return instance.value.(V), nil
		}
	}

	return empty, fnerrors.InternalError("%q: no such injected key", inj.key)
}

type injections struct {
	instances []InjectionInstance
}

type InjectionInstance struct {
	key   string
	value any
}

type MakeInjectionInstance interface {
	MakeInjection() []InjectionInstance
}

func (ij InjectionInstance) MakeInjection() []InjectionInstance {
	return []InjectionInstance{ij}
}

type MakeInjectionInstanceFunc func() []InjectionInstance

func (m MakeInjectionInstanceFunc) MakeInjection() []InjectionInstance {
	return m()
}

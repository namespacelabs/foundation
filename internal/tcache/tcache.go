// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tcache

import (
	"context"
	"sync"

	"namespacelabs.dev/foundation/workspace/tasks"
)

type Cache[V any] struct {
	mu      sync.Mutex
	entries map[string]*result[V]
}

type result[V any] struct {
	once  sync.Once
	value V
	err   error
}

type Deferred[V any] struct {
	action  *tasks.ActionEvent
	produce func(context.Context) (V, error)
	result  result[V]
}

func NewCache[V any]() *Cache[V] {
	return &Cache[V]{
		entries: map[string]*result[V]{},
	}
}

func (c *Cache[V]) Compute(key string, produce func() (V, error)) (V, error) {
	c.mu.Lock()
	v, ok := c.entries[key]
	if !ok {
		v = &result[V]{}
		c.entries[key] = v
	}
	c.mu.Unlock()

	v.once.Do(func() {
		v.value, v.err = produce()
	})

	return v.value, v.err
}

func NewDeferred[V any](action *tasks.ActionEvent, produce func(context.Context) (V, error)) *Deferred[V] {
	return &Deferred[V]{
		action:  action,
		produce: produce,
	}
}

func (d *Deferred[V]) Get(ctx context.Context) (V, error) {
	d.result.once.Do(func() {
		d.result.value, d.result.err = tasks.Return(ctx, d.action, d.produce)
	})

	return d.result.value, d.result.err
}

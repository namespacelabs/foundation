// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package nscloud

import (
	"sync"
	"time"
)

var Expired = time.Unix(0, 0)

type Cache[V any] struct {
	mu      sync.Mutex
	entries map[string]*result[V]
}

type result[V any] struct {
	deadline time.Time
	once     sync.Once
	value    V
	err      error
}

func NewCache[V any]() *Cache[V] {
	return &Cache[V]{
		entries: map[string]*result[V]{},
	}
}

func (c *Cache[V]) Compute(key string, produce func() (V, time.Time, error)) (V, error) {
	now := time.Now()

	c.mu.Lock()
	v, ok := c.entries[key]
	if !ok || (!v.deadline.IsZero() && !now.Before(v.deadline)) {
		v = &result[V]{}
		c.entries[key] = v
	}
	c.mu.Unlock()

	v.once.Do(func() {
		v.value, v.deadline, v.err = produce()
	})

	c.mu.Lock()
	if !time.Now().Before(v.deadline) || v.err != nil {
		delete(c.entries, key)
	}
	c.mu.Unlock()

	return v.value, v.err
}

func DontCache[V any](callback func() (V, error)) func() (V, time.Time, error) {
	return func() (V, time.Time, error) {
		v, err := callback()
		return v, Expired, err
	}
}

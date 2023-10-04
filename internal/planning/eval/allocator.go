// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package eval

import (
	"context"
	"fmt"
	"sync"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
)

type AllocState struct {
	mu        sync.Mutex
	instances map[string]*schema.NeedAllocation
}

type AllocatorFunc func(context.Context, *schema.Node, *schema.Need) (*schema.NeedValue, error)

func NewAllocState() *AllocState {
	return &AllocState{
		instances: map[string]*schema.NeedAllocation{},
	}
}

func (r *AllocState) Alloc(ctx context.Context, server *schema.Server, allocator []AllocatorFunc, n *schema.Node, k int) (*schema.NeedAllocation, error) {
	// Allow for concurrent computations of the same key.
	key := fmt.Sprintf("%s:%s/%d", server.PackageName, n.PackageName, k)

	r.mu.Lock()
	res, ok := r.instances[key]
	r.mu.Unlock()
	if ok {
		return res, nil
	}

	if k >= len(n.GetNeed()) {
		return nil, fnerrors.New("k is too large")
	}

	v, err := r.allocValue(ctx, allocator, n, n.GetNeed()[k])
	if err != nil {
		return nil, err
	}

	vwp := &schema.NeedAllocation{Need: n.GetNeed()[k], Value: v}

	r.mu.Lock()
	r.instances[key] = vwp
	r.mu.Unlock()

	return vwp, nil
}

func (r *AllocState) allocValue(ctx context.Context, allocator []AllocatorFunc, n *schema.Node, need *schema.Need) (*schema.NeedValue, error) {
	for _, alloc := range allocator {
		v, err := alloc(ctx, n, need)
		if err != nil || v != nil {
			return v, err
		}
	}

	return nil, nil
}

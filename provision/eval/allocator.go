// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package eval

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"namespacelabs.dev/foundation/internal/frontend"
	"namespacelabs.dev/foundation/schema"
)

type AllocState struct {
	mu        sync.Mutex
	instances map[string]frontend.ValueWithPath
}

type AllocatorFunc func(context.Context, *schema.Node, *schema.Need) (interface{}, error)

func NewAllocState() *AllocState {
	return &AllocState{
		instances: map[string]frontend.ValueWithPath{},
	}
}

func (r *AllocState) Alloc(ctx context.Context, server *schema.Server, allocator []AllocatorFunc, n *schema.Node, k int) (frontend.ValueWithPath, error) {
	// Allow for concurrent computations of the same key.
	key := fmt.Sprintf("%s:%s/%d", server.PackageName, n.PackageName, k)

	r.mu.Lock()
	res, ok := r.instances[key]
	r.mu.Unlock()
	if ok {
		return res, nil
	}

	if k >= len(n.GetNeed()) {
		return frontend.ValueWithPath{}, errors.New("k is too large")
	}

	v, err := r.allocValue(ctx, allocator, n, n.GetNeed()[k])
	if err != nil {
		return frontend.ValueWithPath{}, err
	}

	vwp := frontend.ValueWithPath{Need: n.GetNeed()[k], Value: v}

	r.mu.Lock()
	r.instances[key] = vwp
	r.mu.Unlock()

	return vwp, nil
}

func (r *AllocState) allocValue(ctx context.Context, allocator []AllocatorFunc, n *schema.Node, need *schema.Need) (interface{}, error) {
	for _, alloc := range allocator {
		v, err := alloc(ctx, n, need)
		if err != nil || v != nil {
			return v, err
		}
	}
	return nil, nil
}
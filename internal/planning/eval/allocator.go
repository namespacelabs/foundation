// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package eval

import (
	"context"
	"fmt"
	"sync"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type AllocState struct {
	mu        sync.Mutex
	instances map[string]pkggraph.ValueWithPath
}

type AllocatorFunc func(context.Context, *schema.Node, *schema.Need) (interface{}, error)

func NewAllocState() *AllocState {
	return &AllocState{
		instances: map[string]pkggraph.ValueWithPath{},
	}
}

func (r *AllocState) Alloc(ctx context.Context, server *schema.Server, allocator []AllocatorFunc, n *schema.Node, k int) (pkggraph.ValueWithPath, error) {
	// Allow for concurrent computations of the same key.
	key := fmt.Sprintf("%s:%s/%d", server.PackageName, n.PackageName, k)

	r.mu.Lock()
	res, ok := r.instances[key]
	r.mu.Unlock()
	if ok {
		return res, nil
	}

	if k >= len(n.GetNeed()) {
		return pkggraph.ValueWithPath{}, fnerrors.New("k is too large")
	}

	v, err := r.allocValue(ctx, allocator, n, n.GetNeed()[k])
	if err != nil {
		return pkggraph.ValueWithPath{}, err
	}

	vwp := pkggraph.ValueWithPath{Need: n.GetNeed()[k], Value: v}

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

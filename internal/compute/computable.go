// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package compute

import (
	"context"
	"reflect"

	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/go-ids"
)

// A computable represents a node in a computation graph. Each computation node produces a value,
// which is computed by its `Compute` method. A node's output can be cached, and is keyed by all
// declared inputs. For correctness, it is required that all meaningful inputs that may impact
// the semantics of the output (and its digest), be listed in `Inputs`. A node can also depend on
// other nodes, and use their computed values. That relationship is established by declaring another
// Computable as an Input to this one. The resulting value, will then be available in `Resolved`.
// If any of node's dependencies fails to compute, the node implicitly fails as well (with the
// same error).
type Computable[V any] interface {
	Action() *tasks.ActionEvent
	Inputs() *In
	Output() Output // Optional.
	Compute(context.Context, Resolved) (V, error)

	UntypedComputable
}

type UntypedComputable interface {
	Action() *tasks.ActionEvent
	Inputs() *In
	Output() Output // Optional.

	// Implementations of Computable must embed one of `LocalScoped`, `DoScoped` or `PrecomputeScoped`.
	prepareCompute(UntypedComputable) computeInstance
}

type computeInstance struct {
	Output
	IsGlobal      bool
	IsPrecomputed bool
	State         *embeddedState
	Computable    UntypedComputable
	OutputType    interface{}
	Compute       func(context.Context, Resolved) (any, error)
}

// A Computable whose lifecycle is bound to the environment where it is computed.
// This is the most common option, as it yields for the clearest cancellation
// semantics and instance-level sharing as expected in Go. A "LocalScoped" Computable
// embeds a Promise which keeps track of the computation state of the Computable.
type LocalScoped[V any] struct{ embeddedState }

// A Computable whose lifecycle is bound to the surrounding Do() invocation. If the
// Computable is attempted to be computed outside a Do()-bound context, the program
// panics. A Do()-scoped Computable _must_ have deterministic inputs, so that a key
// can be calculated. This type of Computable is useful when different parts of the
// program want to share a computation that depends strictly on the inputs.
type DoScoped[V any] struct{}

// This Computable embeds a "precomputed" value and Compute() is guaranteed to return
// immediately.
type PrecomputeScoped[V any] struct{}

func (c *LocalScoped[V]) prepareCompute(rc UntypedComputable) computeInstance {
	opts := prepareInstance[V](rc, false, false)
	opts.State = &c.embeddedState
	return opts
}

func (c DoScoped[V]) prepareCompute(rc UntypedComputable) computeInstance {
	return prepareInstance[V](rc, true, false)
}

func (c PrecomputeScoped[V]) prepareCompute(rc UntypedComputable) computeInstance {
	return prepareInstance[V](rc, false, true)
}

func (c *LocalScoped[V]) Output() Output     { return Output{} }
func (c DoScoped[V]) Output() Output         { return Output{} }
func (c PrecomputeScoped[V]) Output() Output { return Output{} }

type embeddedState struct {
	promise  Promise[any] // Polymorphic.
	running  bool
	uniqueID string // Used in continuous.
}

func (es *embeddedState) ensureUniqueID() string {
	es.promise.mu.Lock()
	defer es.promise.mu.Unlock()
	if es.uniqueID == "" {
		es.uniqueID = ids.NewRandomBase62ID(8)
	}
	return es.uniqueID
}

func prepareInstance[V any](rc UntypedComputable, global, precomputed bool) computeInstance {
	var t *V // Capture the type.

	typed := rc.(Computable[V])

	return computeInstance{
		Output:        typed.Output(),
		IsGlobal:      global,
		IsPrecomputed: precomputed,
		Computable:    rc,
		OutputType:    t,
		Compute: func(ctx context.Context, deps Resolved) (any, error) {
			v, err := typed.Compute(ctx, deps)
			return v, err
		},
	}
}

func (opts computeInstance) Action() *tasks.ActionEvent { return opts.Computable.Action() }
func (opts computeInstance) Inputs() *In                { return opts.Computable.Inputs() }

func (opts computeInstance) CacheInfo() (*cacheable, bool) {
	if opts.IsPrecomputed {
		return nil, false
	}

	cacheable := cacheableFor(opts.OutputType)
	shouldCache := CachingEnabled && opts.CanCache() && cacheable != nil
	return cacheable, shouldCache
}

func (opts computeInstance) NewInstance() interface{} {
	// OutputType is a *V
	typ := reflect.TypeOf(opts.OutputType).Elem()
	if typ.Kind() == reflect.Ptr {
		// If it's a pointer, e.g. a proto pointer, then instantiate the value instead.
		return reflect.New(typ.Elem()).Interface()
	}
	return reflect.New(typ).Elem().Interface()
}

type hasUnwrap interface {
	Unwrap() UntypedComputable
}

func Unwrap(c any) (any, bool) {
	if u, ok := c.(hasUnwrap); ok {
		return u.Unwrap(), true
	}
	return nil, false
}

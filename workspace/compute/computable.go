// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package compute

import (
	"context"
	"reflect"

	"namespacelabs.dev/foundation/workspace/tasks"
)

type Computable[V any] interface {
	Action() *tasks.ActionEvent
	Inputs() *In
	Output() Output // Optional.
	Compute(context.Context, Resolved) (V, error)

	rawComputable
}

type rawComputable interface {
	Action() *tasks.ActionEvent
	Inputs() *In
	Output() Output // Optional.

	// Implementations of Computable must embed one of `LocalScoped`, `DoScoped` or `PrecomputeScoped`.
	prepareCompute(rawComputable) computeInstance
}

type computeInstance struct {
	Output
	IsGlobal      bool
	IsPrecomputed bool
	State         *embeddedState
	Computable    rawComputable
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

func (c *LocalScoped[V]) prepareCompute(rc rawComputable) computeInstance {
	opts := prepareInstance[V](rc, false, false)
	opts.State = &c.embeddedState
	return opts
}

func (c DoScoped[V]) prepareCompute(rc rawComputable) computeInstance {
	return prepareInstance[V](rc, true, false)
}

func (c PrecomputeScoped[V]) prepareCompute(rc rawComputable) computeInstance {
	return prepareInstance[V](rc, false, true)
}

func (c *LocalScoped[V]) Output() Output     { return Output{} }
func (c DoScoped[V]) Output() Output         { return Output{} }
func (c PrecomputeScoped[V]) Output() Output { return Output{} }

type embeddedState struct {
	promise Promise[any] // Polymorphic.
	running bool
}

func prepareInstance[V any](rc rawComputable, global, precomputed bool) computeInstance {
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

func produceValue[V any]() func(context.Context, rawComputable, Resolved) (any, error) {
	return func(ctx context.Context, rc rawComputable, deps Resolved) (any, error) {
		v, err := rc.(Computable[V]).Compute(ctx, deps)
		return v, err
	}
}

type hasUnwrap interface {
	Unwrap() rawComputable
}

func Unwrap(c any) (any, bool) {
	if u, ok := c.(hasUnwrap); ok {
		return u.Unwrap(), true
	}
	return nil, false
}

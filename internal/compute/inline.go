// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package compute

import (
	"context"
	"fmt"
	"io"

	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/tasks"
)

func Map[V any](action *tasks.ActionEvent, inputs *In, output Output,
	compute func(context.Context, Resolved) (V, error)) Computable[V] {
	return &inline[V]{action: action, inputs: inputs, output: output, compute: compute}
}

func Inline[To any](action *tasks.ActionEvent, compute func(context.Context) (To, error)) Computable[To] {
	return &inline[To]{
		action: action,
		inputs: Inputs().Indigestible("inline is not cacheable", "true"),
		output: Output{},
		compute: func(ctx context.Context, _ Resolved) (To, error) {
			return compute(ctx)
		}}
}

func Transform[From, To any](desc string, from Computable[From], compute func(context.Context, From) (To, error)) Computable[To] {
	newAction := from.Action().Clone(func(original string) string {
		return fmt.Sprintf("%s: %s", original, desc)
	})
	return Map(newAction, Inputs().Computable("from", from).Indigestible("transform doesnt trust closures", "true"), Output{
		NotCacheable: true, // There's no value in retaining these intermediary artifacts.
	}, func(ctx context.Context, r Resolved) (To, error) {
		return compute(ctx, MustGetDepValue(r, from, "from"))
	})
}

type inline[V any] struct {
	action  *tasks.ActionEvent
	inputs  *In
	output  Output
	compute func(context.Context, Resolved) (V, error)

	LocalScoped[V]
}

func (in *inline[V]) Action() *tasks.ActionEvent { return in.action }
func (in *inline[V]) Inputs() *In                { return in.inputs }
func (in *inline[V]) Output() Output             { return in.output }
func (in *inline[V]) Compute(ctx context.Context, deps Resolved) (V, error) {
	return in.compute(ctx, deps)
}

type Digestible interface {
	ComputeDigest(context.Context) (schema.Digest, error)
}

type DigestFunc func(context.Context) (schema.Digest, error)

func (f DigestFunc) ComputeDigest(ctx context.Context) (schema.Digest, error) { return f(ctx) }

func Precomputed[V any](v V, computeDigest func(context.Context, V) (schema.Digest, error)) Computable[V] {
	return precomputed[V]{value: v, computeDigest: computeDigest}
}

func IsPrecomputed[V any](c Computable[V]) (V, bool) {
	if v, ok := c.(precomputed[V]); ok {
		return v.value, true
	}

	var empty V
	return empty, false
}

type precomputed[V any] struct {
	value         V
	err           error
	computeDigest func(context.Context, V) (schema.Digest, error)
	PrecomputeScoped[V]
}

var _ Digestible = precomputed[any]{}

func (p precomputed[V]) Action() *tasks.ActionEvent { return nil }
func (p precomputed[V]) Inputs() *In {
	return Inputs().Marshal("digest", func(ctx context.Context, w io.Writer) error {
		digest, err := p.computeDigest(ctx, p.value)
		if err != nil {
			return err
		}
		_, err = fmt.Fprint(w, digest.String())
		return err
	})
}
func (p precomputed[V]) Output() Output { return Output{NotCacheable: true} }
func (p precomputed[V]) Compute(ctx context.Context, deps Resolved) (V, error) {
	return p.value, p.err
}

func (p precomputed[V]) ComputeDigest(ctx context.Context) (schema.Digest, error) {
	return p.computeDigest(ctx, p.value)
}

func Sticky[V any](action *tasks.ActionEvent, c Computable[V]) Computable[V] {
	return &named[V]{action: action, c: c, sticky: true}
}

func Named[V any](action *tasks.ActionEvent, c Computable[V]) Computable[V] {
	return &named[V]{action: action, c: c}
}

type named[V any] struct {
	action *tasks.ActionEvent
	c      Computable[V]
	sticky bool

	LocalScoped[V]
}

func (in *named[V]) Action() *tasks.ActionEvent { return in.action }
func (in *named[V]) Inputs() *In {
	name, _ := tasks.NameOf(in.action)
	inputs := Inputs().Computable(name, in.c)
	if in.sticky {
		inputs.named = in.c
	}
	return inputs
}
func (in *named[V]) Output() Output { return in.c.Output().DontCache() } // Caching here is redundant.
func (in *named[V]) Compute(ctx context.Context, deps Resolved) (V, error) {
	name, _ := tasks.NameOf(in.action)
	return MustGetDepValue(deps, in.c, name), nil
}

func (in *named[V]) Unwrap() UntypedComputable { return in.c }

func Error[V any](err error) Computable[V] {
	return precomputed[V]{err: err}
}

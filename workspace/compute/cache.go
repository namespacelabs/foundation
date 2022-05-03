// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package compute

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/versions"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/cache"
	"namespacelabs.dev/foundation/workspace/tasks"
)

type Cacheable[V any] interface {
	Digester
	LoadCached(context.Context, cache.Cache, CacheableInstance, schema.Digest) (Result[V], error)
	Cache(context.Context, cache.Cache, V) (schema.Digest, error)
}

type CacheableInstance interface {
	NewInstance() interface{}
}

type ComputeDigestFunc func(context.Context, any) (schema.Digest, error)

type cacheable struct {
	Type reflect.Type

	ComputeDigest ComputeDigestFunc
	LoadCached    func(context.Context, cache.Cache, CacheableInstance, schema.Digest) (Result[any], error)
	Cache         func(context.Context, cache.Cache, any) (schema.Digest, error)
}

var cacheables []cacheable

func RegisterCacheable[V any](c Cacheable[V]) {
	var t *V // Capture the type.

	cacheables = append(cacheables, cacheable{
		Type: interfaceType(t),
		ComputeDigest: func(ctx context.Context, v any) (schema.Digest, error) {
			return c.ComputeDigest(ctx, v)
		},
		LoadCached: func(ctx context.Context, cache cache.Cache, t CacheableInstance, d schema.Digest) (Result[any], error) {
			v, err := c.LoadCached(ctx, cache, t, d)
			if err != nil {
				return Result[any]{}, err
			}
			var r Result[any]
			r.Digest = v.Digest
			r.NonDeterministic = v.NonDeterministic
			r.Value = v.Value
			return r, nil
		},
		Cache: func(ctx context.Context, cache cache.Cache, v any) (schema.Digest, error) {
			vtyped, ok := v.(V)
			if !ok {
				return schema.Digest{}, fnerrors.InternalError("failed to cast")
			}
			return c.Cache(ctx, cache, vtyped)
		},
	})
}

func interfaceType(t interface{}) reflect.Type {
	vt := reflect.TypeOf(t)
	if vt.Kind() != reflect.Ptr {
		fnerrors.Panic("expected pointer to type")
	}

	elem := vt.Elem()
	switch elem.Kind() {
	case reflect.Interface, reflect.Slice:
		return elem
	}

	fnerrors.Panic("unexpected type, got " + elem.String())
	return nil
}

type cacheHit struct {
	Input        schema.Digest
	OutputDigest schema.Digest
	Hit          bool // Always set if we have a stored entry that maps to the inputs.
	VerifiedHit  bool // If verification is enabled, only set if we've verified the output matches our expectations. Else same value as Hit.

	Inputs *computedInputs // Set if VerifyCache is true.
}

func checkLoadCache(ctx context.Context, what string, g *Orch, c computeInstance, cacheable *cacheable, computedDigest schema.Digest, p *Promise[any]) (cacheHit, error) {
	var hit cacheHit

	err := c.Action().Clone(func(name string) string { return fmt.Sprintf("%s (%s)", what, name) }).
		Arg("inputs.digest", computedDigest.String()).
		Run(ctx, func(ctx context.Context) error {
			output, cached, err := g.cache.LoadEntry(ctx, computedDigest)
			if err == nil {
				tasks.Attachments(ctx).AddResult("cached", cached)
				trace.SpanFromContext(ctx).SetAttributes(attribute.Bool("cached", cached), attribute.String("outputDigest", output.Digest.String()))
				if !cached {
					return nil
				}
				v, err := cacheable.LoadCached(ctx, g.cache, c, output.Digest)
				if err == nil {
					hit.Hit = true
					hit.OutputDigest = output.Digest

					if VerifyCaching {
						hit.Input = computedDigest
						verifyComputedDigest(ctx, c.Computable, cacheable, v.Value, output.Digest)
						// Don't resolve the promise, so the regular path is triggered.
						return nil
					}

					hit.VerifiedHit = true

					return p.resolve(ResultWithTimestamp[any]{Result: v, Cached: true, Timestamp: output.Timestamp}, nil)
				} else {
					trace.SpanFromContext(ctx).RecordError(err)
				}
			} else {
				trace.SpanFromContext(ctx).RecordError(err)
			}
			return nil
		})

	return hit, err
}

func deferStore(ctx context.Context, g *Orch, c hasAction, cacheable *cacheable, d schema.Digest, ts time.Time, v any, inputs *computedInputs) {
	var pointers []schema.Digest

	if inputs.Digest.IsSet() {
		pointers = append(pointers, inputs.Digest)
	}
	if inputs.PostComputeDigest.IsSet() {
		pointers = append(pointers, inputs.PostComputeDigest)
	}

	if len(pointers) == 0 {
		return
	}

	g.DetachWith(Detach{
		Action:     c.Action().Clone(func(name string) string { return fmt.Sprintf("cache.store (%s)", name) }).LogLevel(1).Arg("digests", pointers),
		BestEffort: true,
		Do: func(ctx context.Context) error {
			result, err := cacheable.Cache(ctx, g.cache, v)
			if err != nil {
				return err
			}

			if VerifyCaching && result != d {
				zerolog.Ctx(ctx).Error().
					Str("type", typeStr(c)).
					Str("cacheableType", typeStr(cacheable)).
					Stringer("got", result).Stringer("expected", d).
					Msg("VerifyCache: source of non-determinism writing to the output cache")
			}

			entry := cache.CachedOutput{
				Digest:       result,
				Timestamp:    ts,
				CacheVersion: versions.CacheVersion,
			}

			entry.Debug.Serial = inputs.serial
			entry.Debug.PackagePath = inputs.pkgPath
			entry.Debug.Typename = inputs.typeName

			entry.InputDigests = map[string]string{}
			for _, kv := range inputs.digests {
				if kv.IsSet {
					entry.InputDigests[kv.Name] = kv.Digest
				}
			}

			return g.cache.StoreEntry(ctx, pointers, entry)
		},
	})
}

func cacheableFor(outputType interface{}) *cacheable {
	vt := reflect.TypeOf(outputType)
	if vt == nil {
		return nil
	}

	if vt.Kind() != reflect.Ptr {
		return nil
	}

	elem := vt.Elem()

	for _, df := range cacheables {
		// We check for both pointers and values as either could implement the
		// registered interface. Protos in particular always use pointers.
		if elem == df.Type || (df.Type.Kind() == reflect.Interface && (vt.Implements(df.Type) || elem.Implements(df.Type))) {
			return &df
		}
	}

	return nil
}

func verifyComputedDigest(ctx context.Context, c rawComputable, cacheable *cacheable, v interface{}, outputDigest schema.Digest) {
	l := zerolog.Ctx(ctx).With().
		Str("cacheableType", typeStr(cacheable)).
		Str("type", typeStr(c)).
		Logger()

	computed, err := cacheable.ComputeDigest(ctx, v)
	if err != nil {
		l.Error().Err(err).
			Msg("VerifyCaching: failed to produce digest to verify")
	} else if computed != outputDigest {
		l.Error().Err(err).
			Stringer("got", computed).Stringer("expected", outputDigest).
			Msg("VerifyCaching: computed digest differs on cache load")
	}
}

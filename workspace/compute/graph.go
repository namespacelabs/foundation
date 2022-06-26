// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package compute

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/cache"
	"namespacelabs.dev/foundation/workspace/tasks"
)

var (
	// Configurable globally only for now.
	CachingEnabled = true
	// If enabled, does not use cached contents, but still verifies that if we do have
	// cached contents, they match what we produced.
	VerifyCaching = false
)

const (
	outputCachingInformation = true

	cleanerFuncLogLevel = 2
)

type contextKey string

var (
	_graphKey = contextKey("fn.workspace.graph")
)

type Orch struct {
	cache   cache.Cache
	origctx context.Context
	exec    executor.Executor

	mu       sync.Mutex
	promises map[string]*Promise[any]
	cleaners []cleaner
}

type cleaner struct {
	ev *tasks.ActionEvent
	f  func(context.Context) error
}

func On(ctx context.Context) *Orch {
	v := ctx.Value(_graphKey)
	if v == nil {
		return nil
	}
	return v.(*Orch)
}

var errNoResult = errors.New("no result")

func NoResult[V any]() (Result[V], error) { return Result[V]{}, errNoResult }

func startComputing(ctx context.Context, g *Orch, c rawComputable) *Promise[any] {
	if g == nil {
		// We panic because this is unexpected.
		panic("no graph in context")
	}

	if c == nil {
		return ErrPromise[any](fnerrors.InternalError("Computable is required"))
	}

	return startComputingWithOpts(ctx, g, c.prepareCompute(c))
}

func startComputingWithOpts(ctx context.Context, g *Orch, opts computeInstance) *Promise[any] {
	if opts.IsPrecomputed {
		// Caller guarantees to not block.
		v, err := opts.Compute(ctx, Resolved{})
		if err != nil {
			return ErrPromise[any](err)
		}
		var r ResultWithTimestamp[any]
		r.Value = v
		if digestible, ok := opts.Computable.(Digestible); ok {
			r.Digest, err = digestible.ComputeDigest(ctx)
			if err != nil {
				return ErrPromise[any](err)
			}
		}
		return valueFuture(r)
	}

	computeInputs := opts.Inputs()
	if computeInputs.named != nil {
		var promise *Promise[any]
		// Skip error checking as we never return an error.
		_ = opts.Action().Run(ctx, func(ctx context.Context) error {
			promise = startComputing(ctx, On(ctx), computeInputs.named)
			return nil
		})
		return promise
	}

	// XXX not really happy with passing ctx here, rather than using the top-level graph
	// context. The attribution will be odd. However, submitting another task at this
	// point is also a stretch.
	inputs, err := computeInputs.computeDigest(ctx, opts.Computable, true)
	if err != nil {
		return ErrPromise[any](err)
	}

	// Any computation which yields is keyed on the same inputs shares its output.
	if inputs.Digest.IsSet() {
		p, isRunning := ensurePromise(g, opts.Computable, inputs)
		if !isRunning {
			deferCompute(g, p, opts, inputs)
		}
		return p
	} else {
		if opts.IsGlobal {
			panic("global node that doesn't have stable inputs: " + reflect.TypeOf(opts.Computable).String())
		}
	}

	opts.State.promise.mu.Lock()
	compute := false
	if !opts.State.running {
		initializePromise(&opts.State.promise, opts.Computable, tasks.NewActionID().String())
		opts.State.running = true
		compute = true
	}
	opts.State.promise.mu.Unlock()

	// If another path is already computing the value, waiting on the returned promise will still block.
	if compute {
		// The return value is ignored here because the promise will be resolved
		// by waitCompute, and thus it's value will be returned below.
		_ = waitCompute(ctx, g, &opts.State.promise, opts, inputs)
	}

	return &opts.State.promise
}

func ensurePromise(g *Orch, c hasAction, inputs *computedInputs) (*Promise[any], bool) {
	g.mu.Lock()
	key := inputs.Digest.String()
	p, isComputing := g.promises[key]
	if !isComputing {
		p = makePromise[any](c, key)
		g.promises[key] = p
	}
	g.mu.Unlock()

	if isComputing {
		// A computation is already running for this inputs; we must return the same
		// instance so we re-use the same action ID; this will provide us with the
		// a link between future (and their waiting time), and the actual computation
		// as the promise stores the action ID.
		// XXX re-do this to just use the digest values themselves for action tracking.
		return p, true
	}

	// Returns false if we're not currently computing this Computable.
	return p, false
}

func deferCompute(g *Orch, p *Promise[any], opts computeInstance, inputs *computedInputs) {
	g.exec.Go(func(ctx context.Context) error {
		return waitCompute(ctx, g, p, opts, inputs)
	})
}

func waitCompute(ctx context.Context, g *Orch, p *Promise[any], opts computeInstance, inputs *computedInputs) error {
	cacheable, shouldCache := opts.CacheInfo()

	// Computables are cacheable (if they don't opt-out, and rely on deterministic inputs and outputs).
	// The cache is a simple a content-addressible filesystem, where a digest of the output points to
	// its contents. A separate index is kept that maps "inputs" digest to output digest. There are two
	// types of input digests: "complete" and "incomplete". "complete" digests are produced by computing
	// recursively the digest of each dependency, provided it itself is complete. A leaf Computable
	// produces "complete" digests if all of its inputs are known and deterministic ahead of time. On
	// the other hand "incomplete" digests are computed using the digest of the output of a Computable
	// which doesn't have deterministic inputs. To ensuring we minimize cost while loading, two index
	// entries are maintained pointing at the output: a "complete" one if available, and the
	// "incomplete" one.

	var resolved *Resolved
	var hits []cacheHit

	ev := opts.Action()
	name, _ := tasks.NameOf(ev)

	if err := ev.ID(p.actionID).RunWithOpts(ctx, tasks.RunOpts{
		Wait: func(ctx context.Context) (bool, error) {
			// If we've already calculated an inputs' digest, then attempt to load from the cache
			// directly. If not, we'll need to wait on our dependencies to determine whether a
			// complete digest is available then.
			hit := checkCache(ctx, g, opts, cacheable, shouldCache, inputs, p)
			if VerifyCaching {
				hits = append(hits, hit)
			}
			if hit.VerifiedHit {
				return true, nil
			}

			// If we come in through the "digest-compute" path, then we've already computed the results.
			results, err := waitDeps(ctx, g, name, inputs.computable)
			if err != nil {
				return false, err
			}

			if outputCachingInformation {
				addOutputsToSpan(ctx, results)
			}

			// Compute a new "inputs" digest based on the resolved future outputs. This
			// provides a stable identifier we can cache on. Used below as well in `deferStore`.
			if err := inputs.Finalize(results); err != nil {
				return false, err
			}

			if outputCachingInformation {
				span := trace.SpanFromContext(ctx)
				span.SetAttributes(attribute.Stringer("fn.inputs.postcompute.digest", inputs.PostComputeDigest))
			}

			if shouldCache && inputs.PostComputeDigest.IsSet() {
				// Errors are ignored in cache loading.
				if hit, err := checkLoadCache(ctx, "cache.load.post", g, opts, cacheable, inputs.PostComputeDigest, p); err == nil && hit.Hit {
					if VerifyCaching {
						hit.Inputs = inputs
						hits = append(hits, hit)
					}
					if hit.VerifiedHit {
						return true, nil
					}
				}
			}

			resolved = &Resolved{
				results: results,
			}

			return false, nil
		},
		Run: func(ctx context.Context) error {
			res, err := compute(ctx, g, opts, cacheable, shouldCache, inputs, *resolved)
			if err != nil {
				return err
			}

			if VerifyCaching {
				verifyCacheHits(ctx, opts.Computable, hits, res.Digest)
			}

			return p.resolve(res, nil)
		},
	}); err != nil {
		return p.fail(err)
	}

	return nil
}

func compute(ctx context.Context, g *Orch, opts computeInstance, cacheable *cacheable, shouldCache bool, inputs *computedInputs, resolved Resolved) (ResultWithTimestamp[any], error) {
	v, err := opts.Compute(ctx, resolved)
	if err != nil {
		return ResultWithTimestamp[any]{}, err
	}

	ts := time.Now()

	var digester ComputeDigestFunc
	if digester == nil && cacheable != nil {
		digester = cacheable.ComputeDigest
	}

	d, err := computeOutputDigest(ctx, digester, v)
	if err != nil {
		d = schema.Digest{} // Ignore errors, but don't cache.
		if VerifyCaching {
			fmt.Fprintf(console.Errors(ctx), "VerifyCache: failed to compute digest for %q: %v", typeStr(opts.Computable), err)
		}
	}

	if shouldCache && d.IsSet() {
		deferStore(ctx, g, opts.Computable, cacheable, d, ts, v, inputs)
	}

	if outputCachingInformation {
		trace.SpanFromContext(ctx).SetAttributes(attribute.Stringer("fn.output.digest", d))
	}

	return ResultWithTimestamp[any]{
		Result: Result[any]{
			Digest:           d,
			NonDeterministic: opts.NonDeterministic,
			Value:            v,
		},
		Timestamp: ts,
	}, nil
}

func checkCache(ctx context.Context, g *Orch, opts computeInstance, cacheable *cacheable, shouldCache bool, inputs *computedInputs, p *Promise[any]) cacheHit {
	if outputCachingInformation {
		addInputsToSpan(ctx, opts.Inputs(), inputs, shouldCache)
	}

	if !shouldCache || !inputs.Digest.IsSet() {
		return cacheHit{}
	}

	// Errors are ignored in cache loading.
	if hit, err := checkLoadCache(ctx, "cache.load.pre", g, opts, cacheable, inputs.Digest, p); err == nil && hit.Hit {
		if VerifyCaching {
			hit.Inputs = inputs
		}
		return hit
	}

	return cacheHit{}
}

func waitDeps(ctx context.Context, g *Orch, desc string, computable map[string]rawComputable) (map[string]ResultWithTimestamp[any], error) {
	if len(computable) == 0 {
		return nil, nil
	}

	var rmu sync.Mutex // Protects resolved and digests.

	// We wait in parallel to create N actions so that the full dependency
	// graph is also visible in the action log. This is a bit wasteful though
	// and should be rethinked.
	eg, _ := executor.Newf(ctx, "compute.wait-deps(%s, %d deps)", desc, len(computable))

	results := map[string]ResultWithTimestamp[any]{}
	for k, d := range computable {
		k := k // Close k.
		d := d // Close d.

		eg.Go(func(ctx context.Context) error {
			res, err := startComputing(ctx, g, d).Future().Wait(ctx)
			if err != nil {
				// Make sure this is reported as one of the dependencies failing, instead of this
				// computation. This will provide for better error reporting.
				return fnerrors.DependencyFailed(k, reflect.TypeOf(d).String(), err)
			}
			rmu.Lock()
			results[k] = res
			rmu.Unlock()
			return nil
		})
	}

	// XXX think through this, we're throwing the same errors all over the place.
	// Probably just want a "dependency didn't compute" error here which feels like
	// a cancellation.
	err := eg.Wait()

	return results, err
}

func computeOutputDigest(ctx context.Context, digester ComputeDigestFunc, v interface{}) (schema.Digest, error) {
	if digester != nil {
		return digester(ctx, v)
	}

	if cd, ok := v.(Digestible); ok {
		return cd.ComputeDigest(ctx)
	}

	return schema.Digest{}, nil
}

func (g *Orch) Detach(ev *tasks.ActionEvent, f func(context.Context) error) {
	g.DetachWith(Detach{
		Action: ev,
		Do:     f,
	})
}

type Detach struct {
	Action     *tasks.ActionEvent
	BestEffort bool
	Do         func(context.Context) error
}

func (g *Orch) DetachWith(d Detach) {
	if g == nil {
		// We panic because this is unexpected.
		panic("running outside of a compute.Do block")
	}

	g.exec.Go(func(ctx context.Context) error {
		err := d.Action.Run(ctx, d.Do)
		if errors.Is(err, context.Canceled) {
			return nil
		}

		if err != nil && d.BestEffort {
			fmt.Fprintf(console.Warnings(ctx), "detach failed: %v\n", err)
			return nil // Ignore errors.
		}

		return err
	})
}

func (g *Orch) Cleanup(ev *tasks.ActionEvent, f func(context.Context) error) {
	// XXX check if Cleanup() is called after we're done.
	g.mu.Lock()
	g.cleaners = append(g.cleaners, cleaner{ev, f})
	g.mu.Unlock()
}

func (g *Orch) Call(callback func(context.Context) error) error {
	errCh := make(chan error)
	g.exec.Go(func(ctx context.Context) error {
		defer close(errCh)
		errCh <- callback(ctx)
		return nil // We never fail the parent computation.
	})
	err, ok := <-errCh
	if !ok {
		return fnerrors.New("call was canceled?")
	}
	return err
}

func WithGraphLifecycle[V any](ctx context.Context, f func(context.Context) (V, error)) (V, error) {
	g := On(ctx)
	if g == nil {
		var empty V
		return empty, fnerrors.New("no graph in context")
	}

	return f(g.origctx)
}

func Cache(ctx context.Context) cache.Cache {
	return On(ctx).cache
}

func Do(parent context.Context, do func(context.Context) error) error {
	parentOrch := On(parent)

	var c cache.Cache
	if parentOrch != nil {
		c = parentOrch.cache
	} else {
		var err error
		c, err = cache.Local()
		if err != nil {
			return err
		}
	}

	g := &Orch{
		cache:    c,
		promises: map[string]*Promise[any]{},
	}
	ctx := context.WithValue(parent, _graphKey, g)
	exec, wait := executor.New(ctx, "compute.Do")
	g.origctx = ctx
	g.exec = exec

	// We execute do in the executor instead of directly, to ensure that error
	// propagation is correct; i.e. if a separate branch ends up failing, we
	// should see that error rather than the context cancelation that do() would
	// otherwise.
	exec.Go(do)

	// Importantly, call `wait` before returning to make sure that any deferred work gets concluded.
	errResult := wait()

	g.mu.Lock()
	cleaners := g.cleaners
	g.cleaners = nil
	g.mu.Unlock()

	// XXX parallelize cleanups.
	// Importantly, graph is not present in the context when calling a cleaner function. And we always
	// run cleaners, regardless of errors above.
	for _, c := range cleaners {
		if err := c.ev.LogLevel(cleanerFuncLogLevel).Run(parent, c.f); err != nil {
			if errResult == nil {
				errResult = err
			}
		}
	}

	return errResult
}

func InternalGetFuture[V any](ctx context.Context, c Computable[V]) *Future[any] {
	return startComputing(ctx, On(ctx), c).Future()
}

func Get[V any](ctx context.Context, c Computable[V]) (ResultWithTimestamp[V], error) {
	promise := startComputing(ctx, On(ctx), c)
	r, err := promise.Future().Wait(ctx)
	if err != nil {
		return ResultWithTimestamp[V]{}, err
	}

	typed, ok := r.Value.(V)
	if !ok {
		panic("how did a Computable produce the wrong type?")
	}

	var rwt ResultWithTimestamp[V]
	rwt.Value = typed
	rwt.Digest = r.Digest
	rwt.Cached = r.Cached
	rwt.NonDeterministic = r.NonDeterministic
	rwt.Timestamp = r.Timestamp
	return rwt, nil
}

func GetValue[V any](ctx context.Context, c Computable[V]) (V, error) {
	v, err := Get(ctx, c)
	return v.Value, err
}

func addInputsToSpan(ctx context.Context, in *In, inputs *computedInputs, shouldCache bool) {
	span := trace.SpanFromContext(ctx)

	for _, input := range in.ins {
		span.SetAttributes(attribute.Bool(fmt.Sprintf("fn.input.%s.undetermined", input.Name), input.Undetermined))
	}

	for _, input := range inputs.digests {
		span.SetAttributes(attribute.String(fmt.Sprintf("fn.input.%s.digest", input.Name), input.Digest))
	}

	span.SetAttributes(attribute.Bool("fn.inputs.nonDeterministic", inputs.nonDeterministic))
	span.SetAttributes(attribute.Bool("fn.shouldCache", shouldCache))
}

func addOutputsToSpan(ctx context.Context, results map[string]ResultWithTimestamp[any]) {
	span := trace.SpanFromContext(ctx)
	for k, res := range results {
		span.SetAttributes(attribute.Stringer(fmt.Sprintf("fn.output.%s.digest", k), res.Digest))
	}
}

func verifyCacheHits(ctx context.Context, c rawComputable, hits []cacheHit, d schema.Digest) {
	for _, hit := range hits {
		if hit.Hit && hit.OutputDigest != d {
			console.JSON(console.Errors(ctx),
				fmt.Sprintf("VerifyCache: found non-determinism evaluating %q", typeStr(c)),
				map[string]interface{}{
					"expected":                hit.OutputDigest,
					"got":                     d,
					"matching":                hit.Input,
					"inputs.digest":           hit.Inputs.Digest,
					"inputs.postDigest":       hit.Inputs.PostComputeDigest,
					"inputs.digests":          hit.Inputs.digests,
					"inputs.nonDeterministic": hit.Inputs.nonDeterministic,
				})

			_ = Explain(ctx, console.Debug(ctx), c)
		}
	}
}

func typeStr(v interface{}) string {
	if v == nil {
		return "(nil)"
	}
	return reflect.TypeOf(v).String()
}

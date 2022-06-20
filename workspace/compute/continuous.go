// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package compute

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/workspace/tasks"
	"namespacelabs.dev/go-ids"
)

// ErrDoneSinking is used as a return value from Updated to indicate that
// no more action will be taken and Sink() should return. It is not
// returned as an error by any function.
var ErrDoneSinking = errors.New("done sinking")

type Sinkable interface {
	Inputs() *In
	Updated(context.Context, Resolved) error
	Cleanup(context.Context) error
}

type Versioned interface {
	Observe(context.Context, func(ResultWithTimestamp[any], bool)) (func(), error)
}

type continuousKey string

const _continuousKey = continuousKey("fn.compute.continuous")

func fromCtx(ctx context.Context) *sinkInvocation {
	v := ctx.Value(_continuousKey)
	if v == nil {
		return nil
	}
	return v.(*sinkInvocation)
}

func Stop(ctx context.Context, err error) {
	if inv := fromCtx(ctx); inv != nil {
		inv.eg.Go(func(ctx context.Context) error {
			return err
		})
	} else {
		panic("not under a Continuously() scope")
	}
}

type TransformErrorFunc func(error) error

// Continuously computes `sinkable` and recomputes it on any tansitive change to `sinkable`'s inputs.
//
// `transformErr` (if not nil) allows to transform (e.g. ignore) encountered errors.
func Continuously(baseCtx context.Context, sinkable Sinkable, transformErr TransformErrorFunc) error {
	g := &sinkInvocation{}
	if transformErr != nil {
		g.transformErr = transformErr
	} else {
		// For nil-safety - use a identity transform function.
		g.transformErr = func(err error) error { return err }
	}
	ctx := context.WithValue(baseCtx, _continuousKey, g)
	// We want all executions under the executor to be able to obtain the current invocation.
	eg, wait := executor.New(ctx)
	g.eg = eg
	g.sink(ctx, sinkable.Inputs(), sinkable.Updated)
	err := wait()

	if err := sinkable.Cleanup(ctx); err != nil {
		fmt.Fprintf(console.Warnings(ctx), "clean failed: %v\n", err)
	}

	if err == ErrDoneSinking {
		return nil
	}

	return err
}

func SpawnCancelableOnContinuously(ctx context.Context, f func(context.Context) error) func() {
	return fromCtx(ctx).eg.GoCancelable(f)
}

type sinkInvocation struct {
	eg executor.Executor

	mu           sync.Mutex
	globals      map[string]*observable
	transformErr TransformErrorFunc
}

func (g *sinkInvocation) sink(ctx context.Context, in *In, updated func(context.Context, Resolved) error) {
	var requiredKeys []string
	for _, kv := range in.ins {
		if _, isComputable := kv.Value.(rawComputable); isComputable {
			requiredKeys = append(requiredKeys, kv.Name)
		}
	}

	if len(requiredKeys) == 0 {
		// XXX Make sure that the error is propagated to the caller.
		_ = updated(ctx, Resolved{})
		return
	}

	var mu sync.Mutex
	var done bool

	invalidations := make(chan observableUpdate)
	rebuilt := func(key string, rwt ResultWithTimestamp[any]) bool {
		mu.Lock()
		wasDone := done
		mu.Unlock()

		if !wasDone {
			invalidations <- observableUpdate{key, rwt}
		}

		return !wasDone // Return true to continue.
	}

	for _, kv := range in.ins {
		c, isComputable := kv.Value.(rawComputable)
		if !isComputable {
			continue
		}

		opts := c.prepareCompute(c)
		if opts.IsGlobal {
			g.ensureGlobalObserver(c, kv.Name, rebuilt)
		} else {
			// XXX this is not quite right; it does not account for the
			// shared scope expectations of the Computable. The side-effect
			// is that it will lead to more work than required.
			o := g.newObserver(c, kv.Name, rebuilt)
			g.eg.Go(o.Loop)
		}
	}

	g.eg.Go(func(ctx context.Context) error {
		var last, pending map[string]ResultWithTimestamp[any]
		var lastRevisions, pendingRevisions map[string]uint64
		var invokeUpdateCh chan bool // will be written to by a Go-routine spawned to call updated.

		defer func() {
			mu.Lock()
			done = true
			mu.Unlock()
			close(invalidations)
		}()

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case newV, ok := <-invalidations:
				if !ok {
					return nil
				}

				if pending == nil {
					if previous, ok := last[newV.key]; ok && previous.Digest.IsSet() && previous.Digest.Equals(newV.result.Digest) {
						// Value didn't change.
						continue
					}

					pending = map[string]ResultWithTimestamp[any]{}
					pendingRevisions = map[string]uint64{}
					// Initialize the next invalidation with the same base values.
					for k, v := range last {
						pending[k] = v
						pendingRevisions[k] = lastRevisions[k]
					}
				}

				if previous, ok := pending[newV.key]; !ok || newV.result.revision > previous.revision ||
					(previous.Digest.IsSet() && !previous.Digest.Equals(newV.result.Digest)) {
					pending[newV.key] = newV.result
				}

			case done := <-invokeUpdateCh:
				invokeUpdateCh = nil
				if done {
					return ErrDoneSinking
				}
			}

			if invokeUpdateCh != nil {
				// Already handling a change, continue listening for updates.
				continue
			}

			// We're complete if all keys we're waiting for are present.
			if !hasAllKeys(pending, requiredKeys) {
				continue
			}

			// We use a channel to signal back that the `Updated` invocation has finished.
			invokeUpdateCh = make(chan bool)
			r := Resolved{results: pending}
			last = pending
			lastRevisions = pendingRevisions
			pending = nil
			pendingRevisions = nil
			g.eg.Go(func(ctx context.Context) error {
				defer close(invokeUpdateCh)

				if err := updated(ctx, r); err != nil {
					if err == ErrDoneSinking {
						invokeUpdateCh <- true
						return nil
					}
					return err
				}

				return nil
			})
		}
	})
}

func hasAllKeys(m map[string]ResultWithTimestamp[any], keys []string) bool {
	for _, key := range keys {
		if _, ok := m[key]; !ok {
			return false
		}
	}
	return true
}

type rebuiltFunc func(string, ResultWithTimestamp[any]) bool

func (g *sinkInvocation) ensureGlobalObserver(c rawComputable, key string, rebuilt rebuiltFunc) {
	g.eg.Go(func(ctx context.Context) error {
		inputs, err := c.Inputs().computeDigest(ctx, c, true)
		if err != nil {
			return err
		}

		if !inputs.Digest.IsSet() {
			panic("global node that doesn't have stable inputs")
		}

		g.mu.Lock()
		globalKey := inputs.Digest.String()
		obs := g.globals[globalKey]
		if obs == nil {
			obs = &observable{inv: g, computable: c.prepareCompute(c)}
			if g.globals == nil {
				g.globals = map[string]*observable{}
			}
			g.globals[globalKey] = obs
			g.eg.Go(obs.Loop)
		}
		g.mu.Unlock()

		// We never handle cancelations because the assumption is that the graph
		// does not change for the duration of the Sink().

		obs.mu.Lock()
		latest := obs.latest
		obs.observers = append(obs.observers, onResult{
			ID: ids.NewRandomBase62ID(8),
			Handle: func(rwt ResultWithTimestamp[any]) bool {
				return rebuilt(key, rwt)
			}},
		)
		obs.mu.Unlock()

		// It's OK to do this without holding obs.mu.Lock as the parent will keep
		// track of the latest revision it has observed, and drop updates with old
		// versions.
		if latest.revision > 0 {
			rebuilt(key, latest)
		}

		return nil
	})
}

func (g *sinkInvocation) newObserver(c rawComputable, key string, rebuilt rebuiltFunc) *observable {
	obs := &observable{inv: g, computable: c.prepareCompute(c)}
	obs.observers = append(obs.observers, onResult{
		ID: ids.NewRandomBase62ID(8),
		Handle: func(rwt ResultWithTimestamp[any]) bool {
			return rebuilt(key, rwt)
		},
	})
	return obs
}

type observable struct {
	inv        *sinkInvocation
	computable computeInstance

	mu             sync.Mutex
	observers      []onResult
	revision       uint64
	latest         ResultWithTimestamp[any]
	listenerCancel func() // The value is `Versioned`, and we're listening to new versions.
}

type observableUpdate struct {
	key    string
	result ResultWithTimestamp[any]
}

func (o *observable) Loop(ctx context.Context) error {
	deplessInputs, err := o.computable.Inputs().computeDigest(ctx, o.computable.Computable, true)
	if err != nil {
		return err
	}

	orch := On(ctx)

	cacheable, shouldCache := o.computable.CacheInfo()

	p := makePromise[any](o.computable.Computable, tasks.NewActionID().String())
	hit := checkCache(ctx, orch, o.computable, cacheable, shouldCache, deplessInputs, p)
	if hit.VerifiedHit {
		o.newValue(ctx, p.resolved.value)
	}

	sinkInputs := Inputs()

	depCount := len(deplessInputs.computable)
	for key, c := range deplessInputs.computable {
		sinkInputs = sinkInputs.Computable(key, c)
	}

	o.inv.sink(ctx, sinkInputs, func(ctx context.Context, resolved Resolved) error {
		if o.computable.IsPrecomputed {
			v, err := o.computable.Compute(ctx, Resolved{})
			if err != nil {
				return err
			}
			var r ResultWithTimestamp[any]
			r.Value = v
			o.newValue(ctx, r)
			return nil
		}

		var inputs *computedInputs

		err := o.computable.Action().RunWithOpts(ctx, tasks.RunOpts{
			Wait: func(ctx context.Context) (bool, error) {
				var err error
				inputs, err = o.computable.Inputs().computeDigest(ctx, o.computable.Computable, true)
				if err != nil {
					return false, err
				}

				if err := inputs.Finalize(resolved.results); err != nil {
					return false, err
				}

				p := makePromise[any](o.computable.Computable, tasks.NewActionID().String())

				if hit, err := checkLoadCache(ctx, "cache.load.post", orch, o.computable, cacheable, inputs.PostComputeDigest, p); err == nil && hit.VerifiedHit {
					o.newValue(ctx, p.resolved.value)
					// Continue listening.
					return true, nil
				}

				return false, nil
			},
			Run: func(ctx context.Context) error {
				if res, err := compute(ctx, orch, o.computable, cacheable, shouldCache, inputs, resolved); err != nil {
					if err = o.inv.transformErr(err); err != nil {
						return err
					}
				} else {
					o.newValue(ctx, res)
				}
				return nil
			},
		})

		if err == nil && depCount == 0 {
			return ErrDoneSinking // No more computables we depend on.
		}
		return err
	})

	return nil
}

func (o *observable) newValue(ctx context.Context, latest ResultWithTimestamp[any]) {
	o.mu.Lock()

	if versioned, ok := latest.Value.(Versioned); ok {
		if o.listenerCancel != nil {
			o.listenerCancel()
			o.listenerCancel = nil
		}
		newListener, err := versioned.Observe(ctx, o.newVersion)
		if err == nil {
			// XXX report errors back
			o.listenerCancel = newListener
		} else {
			fmt.Fprintln(console.Stderr(ctx), "failed to observe changes to value", latest.Digest.String(), err)
		}
	}

	broadcast := o.doUpdate(latest)
	o.mu.Unlock()

	broadcast()
}

func (o *observable) newVersion(result ResultWithTimestamp[any], last bool) {
	o.mu.Lock()
	// XXX new versions are not cached.
	broadcast := o.doUpdate(result)
	if last {
		o.listenerCancel = nil
	}
	o.mu.Unlock()

	broadcast()
}

func (o *observable) doUpdate(result ResultWithTimestamp[any]) func() {
	o.revision++
	result.revision = o.revision
	o.latest = result
	observers := make([]onResult, len(o.observers))
	copy(observers, o.observers) // Make a copy so we can safely iterate when there are concurrent changes.

	// This func should be called without holding `mu`.
	return func() {
		var handledObservers []onResult
		for _, f := range observers {
			if f.Handle(result) {
				handledObservers = append(handledObservers, f)
			}
		}
		// We update observers if any `Handle` func returned false.
		if len(handledObservers) != len(observers) {
			o.mu.Lock()
			o.observers = handledObservers
			if len(o.observers) == 0 && o.listenerCancel != nil {
				// No more observers, cancel the listener.
				o.listenerCancel()
				o.listenerCancel = nil
			}
			o.mu.Unlock()
		}
	}
}

type onResult struct {
	ID     string
	Handle func(ResultWithTimestamp[any]) bool
}

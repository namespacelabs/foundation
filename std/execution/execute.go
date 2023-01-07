// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package execution

import (
	"context"
	"fmt"

	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/schema/orchestration"
	"namespacelabs.dev/foundation/std/tasks"
)

type WaitHandler func(context.Context) (chan *orchestration.Event, func(context.Context) error)

// A waiter implementation is required to close the received channel when it's done.
type Waiter func(context.Context, chan *orchestration.Event) error

type ExecuteOpts struct {
	ContinueOnErrors    bool
	WrapWithActions     bool
	OrchestratorVersion int32

	OnWaiter func(context.Context, Waiter)
}

func Execute(ctx context.Context, actionName string, g *Plan, channelHandler WaitHandler, injected ...MakeInjectionInstance) error {
	return ExecuteExt(ctx, actionName, g, channelHandler, ExecuteOpts{ContinueOnErrors: true}, injected...)
}

func ExecuteExt(ctx context.Context, actionName string, g *Plan, channelHandler WaitHandler, opts ExecuteOpts, injected ...MakeInjectionInstance) error {
	var ch chan *orchestration.Event
	var cleanup func(context.Context) error

	if channelHandler != nil {
		ch, cleanup = channelHandler(ctx)
	}

	eg := executor.New(ctx, "execute.waiters")

	original := opts.OnWaiter
	opts.OnWaiter = func(callCtx context.Context, w Waiter) {
		if original != nil {
			original(callCtx, w)
		}

		eg.Go(func(ctx context.Context) error {
			childCh := make(chan *orchestration.Event)

			eg.Go(func(ctx context.Context) error {
				for ev := range childCh {
					if ch != nil {
						ch <- ev
					} else {
						fmt.Fprintf(console.Debug(ctx), "execute: dropped event\n")
					}

				}

				return nil
			})

			return w(ctx, childCh)
		})
	}

	err := rawExecute(ctx, actionName, g, ch, opts, injected...)

	// Wait for goroutines to complete before closing the channel below.
	waitErr := eg.Wait()
	if err == nil {
		err = waitErr
	}

	if ch != nil {
		close(ch)
	}

	if cleanup != nil {
		cleanupErr := cleanup(ctx)
		if err == nil {
			return cleanupErr
		}
	}

	return err
}

// Don't use this method if you don't have a use-case for it, use Execute.
func RawExecute(ctx context.Context, actionName string, opts ExecuteOpts, g *Plan, injected ...MakeInjectionInstance) error {
	return rawExecute(ctx, actionName, g, nil, opts, injected...)
}

func rawExecute(ctx context.Context, actionName string, g *Plan, ch chan *orchestration.Event, opts ExecuteOpts, injected ...MakeInjectionInstance) error {
	var values []InjectionInstance
	for _, make := range injected {
		values = append(values, make.MakeInjection()...)
	}

	return tasks.Return0(injectValues(ctx, values...), tasks.Action(actionName).Scope(g.scope.PackageNames()...), func(ctx context.Context) error {
		compiled, err := compile(ctx, g.definitions, compileOpts{OrchestratorVersion: opts.OrchestratorVersion})
		if err != nil {
			return err
		}

		return compiled.apply(ctx, ch, opts)
	})
}

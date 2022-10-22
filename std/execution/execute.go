// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package execution

import (
	"context"

	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/orchestration"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/tasks"
)

var (
	ConfigurationInjection = Define[cfg.Configuration]("ns.configuration")
	EnvironmentInjection   = Define[*schema.Environment]("ns.schema.environment")
)

type WaitHandler func(context.Context) (chan *orchestration.Event, func(context.Context) error)

func Execute(ctx context.Context, config cfg.Context, actionName string, g *Plan, channelHandler WaitHandler, injected ...InjectionInstance) error {
	var ch chan *orchestration.Event
	var cleanup func(context.Context) error

	if channelHandler != nil {
		ch, cleanup = channelHandler(ctx)
	}

	waiters, err := rawExecute(ctx, config, actionName, g, ch, injected...)
	if err == nil {
		err = waitMultiple(ctx, waiters, ch)
	} else {
		if ch != nil {
			close(ch)
		}
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
func RawExecute(ctx context.Context, config cfg.Context, actionName string, g *Plan, injected ...InjectionInstance) error {
	_, err := rawExecute(ctx, config, actionName, g, nil, injected...)
	return err
}

func rawExecute(ctx context.Context, env cfg.Context, actionName string, g *Plan, ch chan *orchestration.Event, injected ...InjectionInstance) ([]Waiter, error) {
	injections := append([]InjectionInstance{
		ConfigurationInjection.With(env.Configuration()),
		EnvironmentInjection.With(env.Environment()),
	}, injected...)

	return tasks.Return(injectValues(ctx, injections...), tasks.Action(actionName).Scope(g.scope.PackageNames()...), func(ctx context.Context) ([]Waiter, error) {
		compiled, err := compile(ctx, g.definitions)
		if err != nil {
			return nil, err
		}

		return compiled.apply(ctx, ch)
	})
}

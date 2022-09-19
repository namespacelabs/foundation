// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package ops

import (
	"context"

	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/orchestration"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace/tasks"
)

var (
	ConfigurationInjection = Define[planning.Configuration]("ns.configuration")
	EnvironmentInjection   = Define[*schema.Environment]("ns.schema.environment")
)

type WaitHandler func(context.Context) (chan *orchestration.Event, func(error) error)

func Execute(ctx context.Context, config planning.Context, actionName string, g *Plan, channelHandler WaitHandler, injected ...InjectionInstance) error {
	waiters, err := rawExecute(ctx, config, actionName, g, injected...)
	if err != nil {
		return err
	}

	return WaitMultipleWithHandler(ctx, waiters, channelHandler)
}

// Don't use this method if you don't have a use-case for it, use Execute.
func RawExecute(ctx context.Context, config planning.Context, actionName string, g *Plan, injected ...InjectionInstance) error {
	_, err := rawExecute(ctx, config, actionName, g, injected...)
	return err
}

func rawExecute(ctx context.Context, env planning.Context, actionName string, g *Plan, injected ...InjectionInstance) ([]Waiter, error) {
	injections := append([]InjectionInstance{
		ConfigurationInjection.With(env.Configuration()),
		EnvironmentInjection.With(env.Environment()),
	}, injected...)

	return tasks.Return(injectValues(ctx, injections...), tasks.Action(actionName).Scope(g.scope.PackageNames()...), func(ctx context.Context) ([]Waiter, error) {
		compiled, err := compile(ctx, g.definitions)
		if err != nil {
			return nil, err
		}

		return compiled.apply(ctx)
	})
}

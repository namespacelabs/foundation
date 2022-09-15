// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package ops

import (
	"context"

	"namespacelabs.dev/foundation/schema/orchestration"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace/tasks"
)

type WaitHandler func(context.Context) (chan *orchestration.Event, func(error) error)

func Execute(ctx context.Context, config planning.Configuration, actionName string, g *Plan, channelHandler WaitHandler, injected ...InjectionInstance) error {
	waiters, err := RawExecute(ctx, config, actionName, g, injected...)
	if err != nil {
		return err
	}

	return WaitMultipleWithHandler(ctx, waiters, channelHandler)
}

func RawExecute(ctx context.Context, config planning.Configuration, actionName string, g *Plan, injected ...InjectionInstance) ([]Waiter, error) {
	injections := append([]InjectionInstance{ConfigurationInjection.With(config)}, injected...)

	return tasks.Return(injectValues(ctx, injections...), tasks.Action(actionName).Scope(g.scope.PackageNames()...), func(ctx context.Context) ([]Waiter, error) {
		return g.apply(ctx)
	})
}

// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package ops

import (
	"context"

	"namespacelabs.dev/foundation/schema/orchestration"
	"namespacelabs.dev/foundation/std/planning"
)

type WaitHandler func(context.Context) (chan *orchestration.Event, func(error) error)

func ExecuteAndWait(ctx context.Context, config planning.Configuration, actionName string, g *Plan, channelHandler WaitHandler, injected ...InjectionInstance) error {
	waiters, err := Execute(ctx, config, actionName, g, injected...)
	if err != nil {
		return err
	}

	return WaitMultipleWithHandler(ctx, waiters, channelHandler)
}

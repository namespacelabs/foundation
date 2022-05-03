// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package deploy

import (
	"context"

	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/stack"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/provision/tool/protocol"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/compute"
)

func Shutdown(ctx context.Context, env workspace.WorkspaceEnvironment, servers []provision.Server) error {
	stack, defs0, err := computeShutdown(ctx, env, servers)
	if err != nil {
		return err
	}

	defs1, err := runtime.For(ctx, env).PlanShutdown(ctx, servers, stack.Servers)
	if err != nil {
		return err
	}

	g := ops.NewRunner()
	if err := g.Add(defs1...); err != nil {
		return err
	}

	if err := g.Add(defs0...); err != nil {
		return err
	}

	_, err = g.Apply(ctx, "deploy.shutdown", env)
	return err
}

func computeShutdown(ctx context.Context, env ops.Environment, servers []provision.Server) (*stack.Stack, []*schema.Definition, error) {
	stack, err := stack.Compute(ctx, servers, stack.ProvisionOpts{})
	if err != nil {
		return nil, nil, err
	}

	handlers, err := computeHandlers(ctx, stack)
	if err != nil {
		return nil, nil, err
	}

	computable, err := invokeHandlers(ctx, env, stack, handlers, protocol.Lifecycle_SHUTDOWN)
	if err != nil {
		return nil, nil, err
	}

	result, err := compute.GetValue(ctx, computable)
	if err != nil {
		return nil, nil, err
	}

	return result.Stack, result.Definitions, nil
}

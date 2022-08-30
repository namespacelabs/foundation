// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package startup

import (
	"context"

	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend"
	"namespacelabs.dev/foundation/internal/stack"
	"namespacelabs.dev/foundation/schema"
)

func ComputeConfig(ctx context.Context, env ops.Environment, serverStartupPlan *schema.StartupPlan, deps []*stack.ParsedNode, info frontend.StartupInputs) (*schema.BinaryConfig, error) {
	computed := &schema.BinaryConfig{}

	// For each already loaded configuration, unify the startup args to produce the final startup configuration.
	for _, dep := range deps {
		if err := loadStartupPlan(ctx, env, dep, info, computed); err != nil {
			return nil, fnerrors.Wrapf(dep.Package.Location, err, "computing startup config")
		}
	}

	mergePlan(serverStartupPlan, computed)

	return computed, nil
}

func loadStartupPlan(ctx context.Context, env ops.Environment, dep *stack.ParsedNode, info frontend.StartupInputs, merged *schema.BinaryConfig) error {
	plan, err := dep.ProvisionPlan.Startup.EvalStartup(ctx, env, info, dep.Allocations)
	if err != nil {
		return fnerrors.Wrap(dep.Package.Location, err)
	}

	mergePlan(plan, merged)
	return nil
}

func mergePlan(plan *schema.StartupPlan, merged *schema.BinaryConfig) {
	merged.Args = append(merged.Args, plan.Args...)

	for k, v := range plan.Env {
		merged.Env = append(merged.Env, &schema.BinaryConfig_EnvEntry{Name: k, Value: v})
	}
}

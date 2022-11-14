// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package startup

import (
	"context"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/planning"
	"namespacelabs.dev/foundation/internal/support"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

func ComputeConfig(ctx context.Context, env pkggraph.Context, serverStartupPlan *schema.StartupPlan, deps []*planning.ParsedNode, info pkggraph.StartupInputs) (*schema.BinaryConfig, error) {
	computed := &schema.BinaryConfig{}

	// For each already loaded configuration, unify the startup args to produce the final startup configuration.
	for _, dep := range deps {
		if err := loadStartupPlan(ctx, env, dep, info, computed); err != nil {
			return nil, fnerrors.NewWithLocation(dep.Package.Location, "computing startup config: %w", err)
		}
	}

	if err := mergePlan(serverStartupPlan, computed); err != nil {
		return nil, err
	}

	return computed, nil
}

func loadStartupPlan(ctx context.Context, env pkggraph.Context, dep *planning.ParsedNode, info pkggraph.StartupInputs, merged *schema.BinaryConfig) error {
	plan, err := dep.ProvisionPlan.Startup.EvalStartup(ctx, env, info, dep.Allocations)
	if err != nil {
		return fnerrors.AttachLocation(dep.Package.Location, err)
	}

	return mergePlan(plan, merged)
}

func mergePlan(plan *schema.StartupPlan, merged *schema.BinaryConfig) error {
	if plan.Command != nil {
		merged.Command = plan.Command
	}

	merged.Args = append(merged.Args, plan.Args...)

	var err error
	merged.Env, err = support.MergeEnvs(merged.Env, plan.Env)
	return err
}

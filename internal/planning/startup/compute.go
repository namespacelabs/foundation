// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package startup

import (
	"context"

	"namespacelabs.dev/foundation/framework/rpcerrors/multierr"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/planning"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

func ComputeConfig(ctx context.Context, env pkggraph.Context, serverStartupPlan *schema.StartupPlan, deps []*planning.ParsedNode, info pkggraph.StartupInputs) (*schema.BinaryConfig, error) {
	computed := &schema.BinaryConfig{}

	// For each already loaded configuration, unify the startup args to produce the final startup configuration.
	for _, dep := range deps {
		if err := loadStartupPlan(ctx, env, dep, info, computed); err != nil {
			return nil, fnerrors.Wrapf(dep.Package.Location, err, "computing startup config")
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
		return fnerrors.Wrap(dep.Package.Location, err)
	}

	return mergePlan(plan, merged)
}

func mergePlan(plan *schema.StartupPlan, merged *schema.BinaryConfig) error {
	merged.Args = append(merged.Args, plan.Args...)

	// XXX O(n^2)
	var errs []error
	for _, entry := range plan.Env {
		var err error
		merged.Env, err = runtime.SetEnv(merged.Env, entry)
		if err != nil {
			errs = append(errs, err)
		}
	}

	return multierr.New(errs...)
}

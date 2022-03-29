// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package startup

import (
	"context"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend"
	"namespacelabs.dev/foundation/internal/stack"
	"namespacelabs.dev/foundation/runtime/rtypes"
)

type Computed struct {
	Args []*rtypes.Arg     `json:"args"`
	Env  map[string]string `json:"env"`
}

func ComputeConfig(ctx context.Context, deps []*stack.ParsedNode, info frontend.StartupInputs) (*Computed, error) {
	var computed Computed

	// For each already loaded configuration, unify the startup args to produce the final startup configuration.
	for _, dep := range deps {
		if err := loadStartupPlan(ctx, dep, info, &computed); err != nil {
			return nil, fnerrors.Wrapf(dep.Package.Location, err, "computing startup config")
		}
	}

	return &computed, nil
}

func loadStartupPlan(ctx context.Context, dep *stack.ParsedNode, info frontend.StartupInputs, merged *Computed) error {
	plan, err := dep.ProvisionPlan.Startup.EvalStartup(ctx, info, dep.Allocations)
	if err != nil {
		return fnerrors.Wrap(dep.Package.Location, err)
	}

	return mergePlan(plan, merged)
}

func mergePlan(plan frontend.StartupPlan, merged *Computed) error {
	for k, v := range plan.Args {
		merged.Args = append(merged.Args, &rtypes.Arg{Name: k, Value: v})
	}

	if plan.Env != nil && merged.Env == nil {
		merged.Env = map[string]string{}
	}

	MergeEnvs(merged.Env, plan.Env)

	return nil
}

func MergeEnvs(t map[string]string, src map[string]string) {
	for k, v := range src {
		t[k] = v
	}
}
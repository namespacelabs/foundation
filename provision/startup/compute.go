// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package startup

import (
	"context"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

func ComputeConfig(ctx context.Context, env pkggraph.Context, serverStartupPlan *schema.StartupPlan, deps []*provision.ParsedNode, info pkggraph.StartupInputs) (*schema.BinaryConfig, error) {
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

func loadStartupPlan(ctx context.Context, env pkggraph.Context, dep *provision.ParsedNode, info pkggraph.StartupInputs, merged *schema.BinaryConfig) error {
	plan, err := dep.ProvisionPlan.Startup.EvalStartup(ctx, env, info, dep.Allocations)
	if err != nil {
		return fnerrors.Wrap(dep.Package.Location, err)
	}

	return mergePlan(plan, merged)
}

func mergePlan(plan *schema.StartupPlan, merged *schema.BinaryConfig) error {
	merged.Args = append(merged.Args, plan.Args...)

	// XXX O(n^2)
	for _, entry := range plan.Env {
		for _, existing := range merged.Env {
			if entry.Name == existing.Name {
				if proto.Equal(entry, existing) {
					continue
				}

				return fnerrors.BadInputError("incompatible values being set for env key %q (%v vs %v)", entry.Name, entry, existing)
			}
		}

		merged.Env = append(merged.Env, entry)
	}

	return nil
}

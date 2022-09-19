// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package prepare

import (
	"context"

	"namespacelabs.dev/foundation/engine/compute"
	"namespacelabs.dev/foundation/internal/orchestration"
	"namespacelabs.dev/foundation/runtime/kubernetes"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func PrepareOrchestrator(env planning.Context, kube compute.Computable[*kubernetes.Cluster]) compute.Computable[[]*schema.DevHost_ConfigureEnvironment] {
	return compute.Map(
		tasks.Action("prepare.orchestrator").HumanReadablef("Deploying the Namespace Orchestrator"),
		compute.Inputs().Str("kind", "orchestrator").Computable("runtime", kube).Proto("env", env.Environment()).Proto("workspace", env.Workspace().Proto()),
		compute.Output{NotCacheable: true},
		func(ctx context.Context, deps compute.Resolved) ([]*schema.DevHost_ConfigureEnvironment, error) {
			kube := compute.MustGetDepValue(deps, kube, "runtime")

			if err := PrepareOrchestratorInKube(ctx, env, kube); err != nil {
				return nil, err
			}

			// The ingress produces no unique configuration.
			return nil, nil
		})
}

func PrepareOrchestratorInKube(ctx context.Context, env planning.Context, kube *kubernetes.Cluster) error {
	_, err := orchestration.PrepareOrchestrator(ctx, env.Configuration(), kube, false)
	return err
}

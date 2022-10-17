// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package prepare

import (
	"context"

	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/orchestration"
	"namespacelabs.dev/foundation/runtime/kubernetes"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/std/tasks"
)

func PrepareOrchestrator(env planning.Context, kube compute.Computable[*kubernetes.Cluster], confs ...compute.Computable[[]*schema.DevHost_ConfigureEnvironment]) compute.Computable[[]*schema.DevHost_ConfigureEnvironment] {
	return compute.Map(
		tasks.Action("prepare.orchestrator").HumanReadablef("Deploying the Namespace Orchestrator"),
		compute.Inputs().Str("kind", "orchestrator").Computable("runtime", kube).Computable("conf", compute.Transform("parse results", compute.Collect(tasks.Action("prepare.kubernetes.configs"), confs...),
			func(ctx context.Context, computed []compute.ResultWithTimestamp[[]*schema.DevHost_ConfigureEnvironment]) ([]*schema.DevHost_ConfigureEnvironment, error) {
				var result []*schema.DevHost_ConfigureEnvironment
				for _, conf := range computed {
					result = append(result, conf.Value...)
				}
				return result, nil
			})).Proto("env", env.Environment()).Proto("workspace", env.Workspace().Proto()),
		compute.Output{NotCacheable: true},
		func(ctx context.Context, deps compute.Resolved) ([]*schema.DevHost_ConfigureEnvironment, error) {
			computed, _ := compute.GetDepWithType[[]*schema.DevHost_ConfigureEnvironment](deps, "conf")

			devhost := &schema.DevHost{Configure: computed.Value}

			config, err := planning.MakeConfigurationCompat(env, env.Workspace(), devhost, env.Environment())
			if err != nil {
				return nil, err
			}

			kube := compute.MustGetDepValue(deps, kube, "runtime")

			if err := PrepareOrchestratorInKube(ctx, config, kube); err != nil {
				return nil, err
			}

			// The ingress produces no unique configuration.
			return nil, nil
		})
}

func PrepareOrchestratorInKube(ctx context.Context, config planning.Configuration, kube *kubernetes.Cluster) error {
	_, err := orchestration.PrepareOrchestrator(ctx, config, kube, false)
	return err
}

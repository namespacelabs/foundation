// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package prepare

import (
	"context"

	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes"
	"namespacelabs.dev/foundation/orchestration"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/tasks"
)

func PrepareOrchestrator(env cfg.Context, kube compute.Computable[*kubernetes.Cluster], confs ...compute.Computable[*schema.DevHost_ConfigureEnvironment]) compute.Computable[*schema.DevHost_ConfigureEnvironment] {
	return compute.Map(
		tasks.Action("prepare.orchestrator").HumanReadablef("Deploying the Namespace Orchestrator"),
		compute.Inputs().Str("kind", "orchestrator").Computable("runtime", kube).
			Computable("conf", compute.Transform("parse results",
				compute.Collect(tasks.Action("prepare.kubernetes.configs"), confs...), mergeConfigs)).
			Proto("env", env.Environment()).Proto("workspace", env.Workspace().Proto()),
		compute.Output{NotCacheable: true},
		func(ctx context.Context, deps compute.Resolved) (*schema.DevHost_ConfigureEnvironment, error) {
			devhost, _ := compute.GetDepWithType[*schema.DevHost](deps, "conf")

			config, err := cfg.MakeConfigurationCompat(env, env.Workspace(), devhost.Value, env.Environment())
			if err != nil {
				return nil, err
			}

			kube := compute.MustGetDepValue(deps, kube, "runtime")

			if err := PrepareOrchestratorInKube(ctx, config, kube); err != nil {
				return nil, err
			}

			// The orchestrator produces no unique configuration.
			return nil, nil
		})
}

func PrepareOrchestratorInKube(ctx context.Context, config cfg.Configuration, kube *kubernetes.Cluster) error {
	_, err := orchestration.PrepareOrchestrator(ctx, config, kube, false)
	return err
}

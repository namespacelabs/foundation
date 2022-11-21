// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package orchestration

import (
	"context"

	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/planning"
	"namespacelabs.dev/foundation/internal/planning/deploy"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/execution"
	"namespacelabs.dev/foundation/std/tasks"
)

const orchestratorStateKey = "foundation.orchestration"

var (
	UseOrchestrator              = true
	RenderOrchestratorDeployment = false
)

func RegisterPrepare() {
	if !UseOrchestrator {
		return
	}

	runtime.RegisterPrepare(orchestratorStateKey, func(ctx context.Context, target cfg.Configuration, cluster runtime.Cluster) (any, error) {
		return tasks.Return(ctx, tasks.Action("orchestrator.prepare"), func(ctx context.Context) (any, error) {
			return PrepareOrchestrator(ctx, target, cluster, true)
		})
	})
}

func PrepareOrchestrator(ctx context.Context, targetEnv cfg.Configuration, cluster runtime.Cluster, wait bool) (any, error) {
	env, err := MakeOrchestratorContext(ctx, targetEnv)
	if err != nil {
		return nil, err
	}

	boundCluster, err := cluster.Bind(env)
	if err != nil {
		return nil, err
	}

	focus, err := planning.RequireServer(ctx, env, schema.PackageName(serverPkg))
	if err != nil {
		return nil, err
	}

	plan, err := deploy.PrepareDeployServers(ctx, env, boundCluster.Planner(), focus)
	if err != nil {
		return nil, err
	}

	computed, err := compute.GetValue(ctx, plan)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, connTimeout)
	defer cancel()

	if wait {
		if err := execution.Execute(ctx, env, "orchestrator.deploy", computed.Deployer, deploy.MaybeRenderBlock(env, boundCluster, RenderOrchestratorDeployment), runtime.InjectCluster(boundCluster)...); err != nil {
			return nil, err
		}
	} else {
		if err := execution.RawExecute(ctx, env, "orchestrator.deploy", computed.Deployer, runtime.InjectCluster(boundCluster)...); err != nil {
			return nil, err
		}
	}

	return &RemoteOrchestrator{cluster: boundCluster, server: focus.Proto()}, nil
}

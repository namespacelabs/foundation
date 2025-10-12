// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package orchestration

import (
	"context"

	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/planning"
	"namespacelabs.dev/foundation/internal/planning/deploy"
	"namespacelabs.dev/foundation/internal/planning/secrets"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/orchestration/client"
	"namespacelabs.dev/foundation/orchestration/server/constants"
	"namespacelabs.dev/foundation/schema"
	runtimepb "namespacelabs.dev/foundation/schema/runtime"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/execution"
	"namespacelabs.dev/foundation/std/tasks"
)

var stateless = &runtimepb.Deployable{
	PackageRef:      schema.MakePackageSingleRef(constants.ServerPkg),
	Id:              constants.ServerId,
	Name:            constants.ServerName,
	DeployableClass: string(schema.DeployableClass_STATELESS),
}

func RegisterPrepare() {
	client.RegisterOrchestrator(func(ctx context.Context, target cfg.Configuration, cluster runtime.Cluster) (any, error) {
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

	boundCluster, err := cluster.Bind(ctx, env)
	if err != nil {
		return nil, err
	}

	if err := deployOrchestrator(ctx, env, boundCluster, wait); err != nil {
		return nil, err
	}

	return client.RemoteOrchestrator(boundCluster, stateless), nil

}

func deployOrchestrator(ctx context.Context, env cfg.Context, boundCluster runtime.ClusterNamespace, wait bool) error {
	focus, err := planning.RequireServer(ctx, env, constants.ServerPkg)
	if err != nil {
		return err
	}

	planner, err := runtime.PlannerFor(ctx, env)
	if err != nil {
		return err
	}

	p := planning.Planner{
		Context:  env,
		Runtime:  planner,
		Registry: planner.Registry(),
		Secrets:  secrets.NoSecrets,
	}

	plan, err := deploy.PrepareDeployServers(ctx, p, focus)
	if err != nil {
		return err
	}

	computed, err := compute.GetValue(ctx, plan)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, client.ConnTimeout)
	defer cancel()

	return execute(ctx, env, boundCluster, computed.Deployer, wait)
}

func execute(ctx context.Context, env cfg.Context, boundCluster runtime.ClusterNamespace, plan *execution.Plan, wait bool) error {
	if wait {
		return execution.Execute(ctx, "orchestrator.deploy", plan,
			deploy.MaybeRenderBlock(env, boundCluster, false),
			execution.FromContext(env), runtime.InjectCluster(boundCluster))
	}

	return execution.RawExecute(ctx, "orchestrator.deploy", execution.ExecuteOpts{ContinueOnErrors: true},
		plan, execution.FromContext(env), runtime.InjectCluster(boundCluster))
}

// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package orchestration

import (
	"context"

	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/orchestration/proto"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/provision/deploy"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

const key = "foundation.orchestration"

var (
	UseOrchestrator              = true
	RenderOrchestratorDeployment = false
)

func RegisterPrepare() {
	if !UseOrchestrator {
		return
	}

	runtime.RegisterPrepare(key, func(ctx context.Context, target planning.Context, cluster runtime.Cluster) (any, error) {
		return tasks.Return(ctx, tasks.Action("orchestrator.prepare").Arg("env", target.Environment().Name), func(ctx context.Context) (any, error) {
			return prepare(ctx, target, cluster)
		})
	})
}

func prepare(ctx context.Context, targetEnv planning.Context, cluster runtime.Cluster) (any, error) {
	env, err := makeOrchEnv(ctx, targetEnv)
	if err != nil {
		return nil, err
	}

	boundCluster, err := cluster.Bind(env)
	if err != nil {
		return nil, err
	}

	focus, err := provision.RequireServer(ctx, env, schema.PackageName(serverPkg))
	if err != nil {
		return nil, err
	}

	plan, err := deploy.PrepareDeployServers(ctx, env, boundCluster.Planner(), []provision.Server{focus}, nil)
	if err != nil {
		return nil, err
	}

	computed, err := compute.GetValue(ctx, plan)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, connTimeout)
	defer cancel()

	if err := ops.ExecuteAndWait(ctx, env.Configuration(), "orchestrator.deploy", computed.Deployer, deploy.MaybeRenderWait(env, cluster, RenderOrchestratorDeployment), runtime.ClusterInjection.With(cluster)); err != nil {
		return nil, err
	}

	var endpoint *schema.Endpoint
	for _, e := range computed.ComputedStack.Endpoints {
		if e.ServerOwner != serverPkg {
			continue
		}

		for _, m := range e.ServiceMetadata {
			if m.Kind == proto.OrchestrationService_ServiceDesc.ServiceName {
				endpoint = e
			}
		}
	}

	if endpoint == nil {
		return nil, fnerrors.InternalError("orchestration service not found: %+v", computed.ComputedStack.Endpoints)
	}

	return &RemoteOrchestrator{cluster: boundCluster, server: focus.Proto(), endpoint: endpoint}, nil
}

// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package orchestration

import (
	"context"
	"fmt"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/planning"
	"namespacelabs.dev/foundation/internal/planning/deploy"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/orchestration/proto"
	"namespacelabs.dev/foundation/schema"
	runtimepb "namespacelabs.dev/foundation/schema/runtime"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/execution"
	"namespacelabs.dev/foundation/std/tasks"
)

const (
	orchestratorStateKey                    = "foundation.orchestration"
	serverId                                = "0fomj22adbua2u0ug3og"
	serverName                              = "orchestration-api-server"
	serverPkg            schema.PackageName = "namespacelabs.dev/foundation/orchestration/server"
	toolPkg              schema.PackageName = "namespacelabs.dev/foundation/orchestration/server/tool"
)

var (
	UseOrchestrator              = true
	UsePinnedOrchestrator        = true
	RenderOrchestratorDeployment = false
	ForceOrchestratorDeployment  = false

	server = &runtimepb.Deployable{
		PackageName:     serverPkg.String(),
		Id:              serverId,
		Name:            serverName,
		DeployableClass: string(schema.DeployableClass_STATEFUL),
	}
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
	versions, err := getVersions(ctx, targetEnv, cluster)
	if err != nil {
		return nil, err
	}

	env, err := MakeOrchestratorContext(ctx, targetEnv, versions.Pinned...)
	if err != nil {
		return nil, err
	}

	boundCluster, err := cluster.Bind(env)
	if err != nil {
		return nil, err
	}

	if err := ensureDeployment(ctx, env, versions, boundCluster, wait); err != nil {
		return nil, err
	}

	return &RemoteOrchestrator{cluster: boundCluster, server: server}, nil
}

func ensureDeployment(ctx context.Context, env cfg.Context, versions *proto.GetOrchestratorVersionResponse, boundCluster runtime.ClusterNamespace, wait bool) error {
	if versions.Current != nil && !ForceOrchestratorDeployment {
		for _, p := range versions.Pinned {
			if p.PackageName == versions.Current.PackageName &&
				p.Repository == versions.Current.Repository &&
				p.Digest == versions.Current.Digest {
				// Current orchestrator already runs the pinned version.
				return nil
			}
		}
	}

	focus, err := planning.RequireServer(ctx, env, schema.PackageName(serverPkg))
	if err != nil {
		return err
	}

	plan, err := deploy.PrepareDeployServers(ctx, env, boundCluster.Planner(), focus)
	if err != nil {
		return err
	}

	computed, err := compute.GetValue(ctx, plan)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, connTimeout)
	defer cancel()

	if wait {
		if err := execution.Execute(ctx, env, "orchestrator.deploy", computed.Deployer, deploy.MaybeRenderBlock(env, boundCluster, RenderOrchestratorDeployment), runtime.InjectCluster(boundCluster)...); err != nil {
			return err
		}
	} else {
		if err := execution.RawExecute(ctx, env, "orchestrator.deploy", computed.Deployer, runtime.InjectCluster(boundCluster)...); err != nil {
			return err
		}
	}

	return nil
}

func getVersions(ctx context.Context, env cfg.Configuration, cluster runtime.Cluster) (*proto.GetOrchestratorVersionResponse, error) {
	if !UsePinnedOrchestrator {
		return &proto.GetOrchestratorVersionResponse{}, nil
	}

	if res, err := getVersionsFromOrchestrator(ctx, env, cluster); err == nil {
		return res, nil
	} else {
		fmt.Fprintf(console.Debug(ctx), "failed to fetch version from orchestrator: %v\nFalling back to pinned version.\n", err)
	}

	// Fallback path: No orchestrator deployed - fetch pinned version directly.
	prebuilts, err := fnapi.GetLatestPrebuilts(ctx, serverPkg, toolPkg)
	if err != nil {
		return nil, err
	}

	res := &proto.GetOrchestratorVersionResponse{}
	for _, prebuilt := range prebuilts.Prebuilt {
		res.Pinned = append(res.Pinned, &schema.Workspace_BinaryDigest{
			PackageName: prebuilt.PackageName,
			Repository:  prebuilt.Repository,
			Digest:      prebuilt.Digest,
		})
	}

	return res, nil
}
func getVersionsFromOrchestrator(ctx context.Context, targetEnv cfg.Configuration, cluster runtime.Cluster) (*proto.GetOrchestratorVersionResponse, error) {
	env, err := MakeOrchestratorContext(ctx, targetEnv)
	if err != nil {
		return nil, err
	}

	boundCluster, err := cluster.Bind(env)
	if err != nil {
		return nil, err
	}

	// Only dial once.
	conn, err := boundCluster.DialServer(ctx, server, &schema.Endpoint_Port{Name: portName})
	if err != nil {
		return nil, err
	}

	rpc, err := grpc.DialContext(ctx, "orchestrator",
		grpc.WithBlock(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return conn, nil
		}))
	if err != nil {
		return nil, err
	}

	return proto.NewOrchestrationServiceClient(rpc).GetOrchestratorVersion(ctx, &proto.GetOrchestratorVersionRequest{})
}

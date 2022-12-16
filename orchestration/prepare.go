// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package orchestration

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"net"
	"time"

	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs/tarfs"
	"namespacelabs.dev/foundation/internal/planning"
	"namespacelabs.dev/foundation/internal/planning/deploy"
	"namespacelabs.dev/foundation/internal/runtime"
	orchestrationpb "namespacelabs.dev/foundation/orchestration/proto"
	"namespacelabs.dev/foundation/orchestration/server/constants"
	"namespacelabs.dev/foundation/schema"
	runtimepb "namespacelabs.dev/foundation/schema/runtime"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/execution"
	"namespacelabs.dev/foundation/std/tasks"
)

const (
	orchestratorStateKey = "foundation.orchestration"
	orchDialTimeout      = 5 * time.Second
	deployPlanFile       = "deployplan.binarypb"
)

var (
	UseOrchestrator              = true
	UseHeadOrchestrator          = false
	SkipVersionCache             = false
	RenderOrchestratorDeployment = false

	stateless = &runtimepb.Deployable{
		PackageName:     constants.ServerPkg.String(),
		PackageRef:      schema.MakePackageSingleRef(constants.ServerPkg),
		Id:              constants.ServerId,
		Name:            constants.ServerName,
		DeployableClass: string(schema.DeployableClass_STATELESS),
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
	env, err := MakeOrchestratorContext(ctx, targetEnv)
	if err != nil {
		return nil, err
	}

	boundCluster, err := cluster.Bind(ctx, env)
	if err != nil {
		return nil, err
	}

	if UseHeadOrchestrator {
		if err := deployHead(ctx, env, boundCluster, wait); err != nil {
			return nil, err
		}

		return &RemoteOrchestrator{cluster: boundCluster, server: stateless}, nil
	}

	versions, err := getVersions(ctx, env, boundCluster)
	if err != nil {
		fmt.Fprintf(console.Debug(ctx), "failed to check if orchestrator is up to date: %v\nwill update orchestrator by default\n", err)
	}

	if versions.GetCurrent() != 0 && versions.GetCurrent() == versions.GetLatest() {
		// already up to date
		return &RemoteOrchestrator{cluster: boundCluster, server: stateless}, nil
	}

	plans, err := fnapi.GetLatestDeployPlans(ctx, constants.ServerPkg)
	if err != nil {
		return nil, err
	}

	for _, plan := range plans.Plan {
		if plan.PackageName != constants.ServerPkg.String() {
			continue
		}

		// Best-effort clean up old stateful set for each orchestrator update.
		// Ideally, we'd only do so when upgrading to the first stateless version.
		// Due to a bug, some users are left with both versions, so we need to delete more aggressively for a while.
		stateful := stateless
		stateful.DeployableClass = string(schema.DeployableClass_STATEFUL)
		if err := boundCluster.DeleteDeployable(ctx, stateful); err != nil {
			fmt.Fprintf(console.Debug(ctx), "failed to delete old orchestrator: %v\n", err)
		}

		if err := deployPlan(ctx, env, plan.Repository, plan.Digest, boundCluster, wait); err != nil {
			return nil, err
		}

		return &RemoteOrchestrator{cluster: boundCluster, server: stateless}, nil
	}

	return nil, fnerrors.InternalError("Did not receive any pinned deployment plan for Namespace orchestrator")
}

func deployHead(ctx context.Context, env cfg.Context, boundCluster runtime.ClusterNamespace, wait bool) error {
	focus, err := planning.RequireServer(ctx, env, constants.ServerPkg)
	if err != nil {
		return err
	}

	planner, err := runtime.PlannerFor(ctx, env)
	if err != nil {
		return err
	}

	plan, err := deploy.PrepareDeployServers(ctx, env, focus.SealedContext(), planner, focus)
	if err != nil {
		return err
	}

	computed, err := compute.GetValue(ctx, plan)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, connTimeout)
	defer cancel()

	return execute(ctx, env, boundCluster, computed.Deployer, wait)
}

func deployPlan(ctx context.Context, env cfg.Context, repository, digest string, boundCluster runtime.ClusterNamespace, wait bool) error {
	plan, err := tasks.Return(ctx, tasks.Action("orchestrator.fetch-latest"), func(ctx context.Context) (*schema.DeployPlan, error) {
		image, err := compute.GetValue(ctx, oci.ImageP(fmt.Sprintf("%s@%s", repository, digest), nil, oci.ResolveOpts{}))
		if err != nil {
			return nil, err
		}

		fsys := tarfs.FS{TarStream: func() (io.ReadCloser, error) { return mutate.Extract(image), nil }}

		data, err := fs.ReadFile(fsys, deployPlanFile)
		if err != nil {
			return nil, err
		}

		any := &anypb.Any{}
		if err := proto.Unmarshal(data, any); err != nil {
			return nil, fnerrors.InternalError("orchestrator: failed to unmarshal %q: %w", deployPlanFile, err)
		}

		plan := &schema.DeployPlan{}
		if err := any.UnmarshalTo(plan); err != nil {
			return nil, fnerrors.InternalError("orchestrator: failed to any to plan %q: %w", deployPlanFile, err)
		}

		return plan, nil
	})

	if err != nil {
		return err
	}

	p := execution.NewPlan(plan.Program.Invocation...)

	return execute(ctx, env, boundCluster, p, wait)
}

func execute(ctx context.Context, env cfg.Context, boundCluster runtime.ClusterNamespace, plan *execution.Plan, wait bool) error {
	if wait {
		return execution.Execute(ctx, "orchestrator.deploy", plan,
			deploy.MaybeRenderBlock(env, boundCluster, RenderOrchestratorDeployment),
			execution.FromContext(env), runtime.InjectCluster(boundCluster))
	}

	return execution.RawExecute(ctx, "orchestrator.deploy", plan, execution.FromContext(env), runtime.InjectCluster(boundCluster))
}

func getVersions(ctx context.Context, env cfg.Context, boundCluster runtime.ClusterNamespace) (*orchestrationpb.GetOrchestratorVersionResponse, error) {
	return tasks.Return(ctx, tasks.Action("orchestrator.check-version"), func(ctx context.Context) (*orchestrationpb.GetOrchestratorVersionResponse, error) {
		ctx, cancel := context.WithTimeout(ctx, orchDialTimeout)
		defer cancel()

		// Only dial once.
		conn, err := boundCluster.DialServer(ctx, stateless, &schema.Endpoint_Port{Name: portName})
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

		return orchestrationpb.NewOrchestrationServiceClient(rpc).GetOrchestratorVersion(ctx, &orchestrationpb.GetOrchestratorVersionRequest{
			SkipCache: SkipVersionCache,
		})
	})
}

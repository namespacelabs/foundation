// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package orchestration

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/planning"
	"namespacelabs.dev/foundation/internal/planning/deploy"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/orchestration/proto"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/execution"
	"namespacelabs.dev/foundation/std/tasks"
)

const (
	orchestratorStateKey                    = "foundation.orchestration"
	serverPkg            schema.PackageName = "namespacelabs.dev/foundation/orchestration/server"
	toolPkg              schema.PackageName = "namespacelabs.dev/foundation/orchestration/server/tool"
)

var (
	UsePinnedOrchestrator        = true
	UseOrchestrator              = true
	ForceOrchestratorDeployment  = false
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
	prebuilts, err := fetchPrebuilts(ctx)
	if err != nil {
		return nil, err
	}

	env, err := MakeOrchestratorContext(ctx, targetEnv, prebuilts)
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

	requiresUpdate, err := requiresUpdate(ctx, cluster, focus, prebuilts)
	if err != nil {
		return nil, err
	}

	if requiresUpdate {
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
	}

	var endpoint *schema.Endpoint
	for _, e := range computed.ComputedStack.Endpoints {
		if !serverPkg.Equals(e.ServerOwner) {
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

func requiresUpdate(ctx context.Context, cluster runtime.Cluster, server planning.Server, prebuilts []*schema.Workspace_BinaryDigest) (bool, error) {
	if ForceOrchestratorDeployment {
		return true, nil
	}

	var expectedImage string
	for _, p := range prebuilts {
		if p.PackageName == string(serverPkg) {
			imgid := oci.ImageID{Repository: p.Repository, Digest: p.Digest}
			expectedImage = imgid.String()
			break
		}
	}

	if expectedImage == "" {
		// No pinned image - always redeploy
		return true, nil
	}

	if kubeCluster, ok := cluster.(kubedef.KubeCluster); ok {
		list, err := kubeCluster.PreparedClient().Clientset.CoreV1().Pods(kubedef.AdminNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: kubedef.SerializeSelector(kubedef.SelectById(server.Proto())),
		})
		if err != nil {
			return false, err
		}

		for _, pod := range list.Items {
			if pod.Status.Phase != corev1.PodRunning {
				continue
			}

			for _, ctr := range pod.Spec.Containers {
				if ctr.Image == expectedImage {
					// orch server with same version already live.
					return false, nil
				}
			}
		}
	}

	return true, nil
}

func fetchPrebuilts(ctx context.Context) ([]*schema.Workspace_BinaryDigest, error) {
	var prebuilts []*schema.Workspace_BinaryDigest

	if UsePinnedOrchestrator {
		res, err := fnapi.GetLatestPrebuilts(ctx, serverPkg, toolPkg)
		if err != nil {
			return nil, err
		}

		for _, prebuilt := range res.Prebuilt {
			prebuilts = append(prebuilts, &schema.Workspace_BinaryDigest{
				PackageName: prebuilt.PackageName,
				Repository:  prebuilt.Repository,
				Digest:      prebuilt.Digest,
			})
		}
	}

	return prebuilts, nil
}

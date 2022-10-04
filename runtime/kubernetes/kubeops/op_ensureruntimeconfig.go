// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubeops

import (
	"context"
	"encoding/json"

	"google.golang.org/protobuf/types/known/wrapperspb"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"namespacelabs.dev/foundation/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/provision/deploy"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/tasks"
)

const runtimeConfigVersion = 0

func registerEnsureRuntimeConfig() {
	ops.RegisterFuncs(ops.Funcs[*kubedef.OpEnsureRuntimeConfig]{
		Handle: func(ctx context.Context, inv *schema.SerializedInvocation, ensure *kubedef.OpEnsureRuntimeConfig) (*ops.HandleResult, error) {
			action := tasks.Action("kubernetes.ensure-runtime-config").
				Scope(schema.PackageName(ensure.Deployable.PackageName)).
				Arg("deployable", ensure.Deployable.PackageName).
				HumanReadablef(inv.Description)

			return tasks.Return(ctx, action, func(ctx context.Context) (*ops.HandleResult, error) {
				data := map[string]string{}

				if ensure.RuntimeConfig != nil {
					serializedConfig, err := json.Marshal(ensure.RuntimeConfig)
					if err != nil {
						return nil, fnerrors.InternalError("failed to serialize runtime configuration: %w", err)
					}
					data["runtime.json"] = string(serializedConfig)
				}

				if len(ensure.Dependency) > 0 {
					resourceData, err := deploy.BuildResourceMap(ctx, ensure.Dependency)
					if err != nil {
						return nil, err
					}

					serializedConfig, err := json.Marshal(resourceData)
					if err != nil {
						return nil, fnerrors.InternalError("failed to serialize resource configuration: %w", err)
					}
					data["resources.json"] = string(serializedConfig)
				}

				if len(data) == 0 {
					return nil, nil
				}

				configDigest, err := schema.DigestOf(runtimeConfigVersion, data["runtime.json"], data["resources.json"])
				if err != nil {
					return nil, fnerrors.InternalError("failed to digest runtime configuration: %w", err)
				}

				deploymentId := kubedef.MakeDeploymentId(ensure.Deployable)
				configId := kubedef.MakeVolumeName(deploymentId, "rtconfig-"+configDigest.Hex[:8])

				cluster, err := kubedef.InjectedKubeClusterNamespace(ctx)
				if err != nil {
					return nil, err
				}

				annotations := kubedef.MakeAnnotations(cluster.KubeConfig().Environment, schema.PackageName(ensure.Deployable.PackageName))
				labels := kubedef.MakeLabels(cluster.KubeConfig().Environment, ensure.Deployable)

				if _, err := cluster.Cluster().(kubedef.KubeCluster).PreparedClient().Clientset.CoreV1().
					ConfigMaps(cluster.KubeConfig().Namespace).
					Apply(ctx,
						applycorev1.ConfigMap(configId, cluster.KubeConfig().Namespace).
							WithAnnotations(annotations).
							WithLabels(labels).
							WithLabels(map[string]string{
								kubedef.K8sKind: kubedef.K8sRuntimeConfigKind,
							}).
							WithImmutable(true).
							WithData(data), kubedef.Ego()); err != nil {
					return nil, err
				}

				return &ops.HandleResult{
					Outputs: []ops.Output{
						{InstanceID: kubedef.RuntimeConfigOutput(ensure.Deployable), Message: &wrapperspb.StringValue{Value: configId}},
					},
				}, nil
			})
		},

		PlanOrder: func(ensure *kubedef.OpEnsureRuntimeConfig) (*schema.ScheduleOrder, error) {
			return nil, nil
		},
	})
}

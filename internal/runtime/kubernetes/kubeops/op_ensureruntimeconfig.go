// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubeops

import (
	"context"
	"encoding/json"

	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/framework/rpcerrors/multierr"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/planning/deploy"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/execution"
	"namespacelabs.dev/foundation/std/tasks"
)

const runtimeConfigVersion = 0

func registerEnsureRuntimeConfig() {
	execution.RegisterFuncs(execution.Funcs[*kubedef.OpEnsureRuntimeConfig]{
		Handle: func(ctx context.Context, inv *schema.SerializedInvocation, ensure *kubedef.OpEnsureRuntimeConfig) (*execution.HandleResult, error) {
			action := tasks.Action("kubernetes.ensure-runtime-config").
				Scope(schema.PackageName(ensure.Deployable.PackageName)).
				Arg("deployable", ensure.Deployable.PackageName).
				HumanReadablef(inv.Description)

			return tasks.Return(ctx, action, func(ctx context.Context) (*execution.HandleResult, error) {
				data := map[string]string{}

				output := &kubedef.EnsureRuntimeConfigOutput{}

				// We here generate the runtime and resource configuration which
				// is injected to servers. Ideally this configmap would be
				// immutable, but in order to minimize churn we update buildvcs
				// in place, if one is available.
				//
				// The name of the configmap is derived from the contents of the
				// runtime and resource configurations. If these don't change,
				// then the resulting configmap will be the same (albeit with an
				// updated buildvcs).

				hashInputs := map[string]any{
					"version": runtimeConfigVersion,
				}

				if ensure.RuntimeConfig != nil {
					serializedConfig, err := json.Marshal(ensure.RuntimeConfig)
					if err != nil {
						return nil, fnerrors.InternalError("failed to serialize runtime configuration: %w", err)
					}
					data["runtime.json"] = string(serializedConfig)
					hashInputs["runtime.json"] = string(serializedConfig)
					output.SerializedRuntimeJson = string(serializedConfig)
				}

				if ensure.BuildVcs != nil {
					serializedConfig, err := json.Marshal(ensure.BuildVcs)
					if err != nil {
						return nil, fnerrors.InternalError("failed to serialize runtime configuration: %w", err)
					}
					// Deliberately not an hash input.
					data["buildvcs.json"] = string(serializedConfig)
				}

				resourceData, err := deploy.BuildResourceMap(ctx, ensure.Dependency)
				if err != nil {
					return nil, err
				}

				if len(ensure.InjectResource) > 0 {
					if resourceData == nil {
						resourceData = map[string]deploy.RawJSONObject{}
					}

					var errs []error
					for _, injected := range ensure.InjectResource {
						var m deploy.RawJSONObject
						if err := json.Unmarshal(injected.SerializedJson, &m); err != nil {
							errs = append(errs, err)
						} else {
							resourceData[injected.GetResourceRef().Canonical()] = m
						}
					}

					if err := multierr.New(errs...); err != nil {
						return nil, fnerrors.InternalError("failed to handle injected resources: %w", err)
					}
				}

				if len(resourceData) > 0 {
					serializedConfig, err := json.Marshal(resourceData)
					if err != nil {
						return nil, fnerrors.InternalError("failed to serialize resource configuration: %w", err)
					}
					data["resources.json"] = string(serializedConfig)
					hashInputs["resources.json"] = string(serializedConfig)
					output.SerializedResourceJson = string(serializedConfig)
				}

				if len(data) > 0 && ensure.PersistConfiguration {
					configDigest, err := schema.DigestOf(hashInputs)
					if err != nil {
						return nil, fnerrors.InternalError("failed to digest runtime configuration: %w", err)
					}

					deploymentId := kubedef.MakeDeploymentId(ensure.Deployable)
					configId := kubedef.MakeResourceName(deploymentId, configDigest.Hex[:8], "runtimecfg", "namespace", "so")

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
								WithData(data), kubedef.Ego()); err != nil {
						return nil, err
					}

					output.ConfigId = configId
				}

				return &execution.HandleResult{
					Outputs: []execution.Output{
						{InstanceID: kubedef.RuntimeConfigOutput(ensure.Deployable), Message: output},
					},
				}, nil
			})
		},

		PlanOrder: func(ctx context.Context, ensure *kubedef.OpEnsureRuntimeConfig) (*schema.ScheduleOrder, error) {
			cluster, err := kubedef.InjectedKubeClusterNamespace(ctx)
			if err != nil {
				return nil, err
			}

			return &schema.ScheduleOrder{
				SchedAfterCategory: []string{kubedef.MakeNamespaceCat(cluster.KubeConfig().Namespace)},
			}, nil
		},
	})
}

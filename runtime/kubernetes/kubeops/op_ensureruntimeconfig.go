// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubeops

import (
	"context"
	"encoding/json"

	"k8s.io/apimachinery/pkg/runtime/schema"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"namespacelabs.dev/foundation/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	fnschema "namespacelabs.dev/foundation/schema"
)

func registerEnsureRuntimeConfig() {
	ops.RegisterFuncs(ops.Funcs[*kubedef.OpEnsureRuntimeConfig]{
		Handle: func(ctx context.Context, _ *fnschema.SerializedInvocation, ensure *kubedef.OpEnsureRuntimeConfig) (*ops.HandleResult, error) {
			data := map[string]string{}

			if ensure.RuntimeConfig != nil {
				serializedConfig, err := json.Marshal(ensure.RuntimeConfig)
				if err != nil {
					return nil, fnerrors.InternalError("failed to serialize runtime configuration: %w", err)
				}
				data["runtime.json"] = string(serializedConfig)
			}

			if len(data) == 0 {
				return nil, nil
			}

			cluster, err := kubedef.InjectedKubeClusterNamespace(ctx)
			if err != nil {
				return nil, err
			}

			annotations := kubedef.MakeAnnotations(cluster.KubeConfig().Environment, fnschema.PackageName(ensure.Deployable.PackageName))
			labels := kubedef.MakeLabels(cluster.KubeConfig().Environment, ensure.Deployable)

			if _, err := cluster.Cluster().(kubedef.KubeCluster).PreparedClient().Clientset.CoreV1().
				ConfigMaps(cluster.KubeConfig().Namespace).
				Apply(ctx,
					applycorev1.ConfigMap(ensure.ConfigId, cluster.KubeConfig().Namespace).
						WithAnnotations(annotations).
						WithLabels(labels).
						WithLabels(map[string]string{
							kubedef.K8sKind: kubedef.K8sRuntimeConfigKind,
						}).
						WithImmutable(true).
						WithData(data), kubedef.Ego()); err != nil {
				return nil, err
			}

			return nil, nil
		},

		PlanOrder: func(ensure *kubedef.OpEnsureRuntimeConfig) (*fnschema.ScheduleOrder, error) {
			return kubedef.PlanOrder(schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}), nil
		},
	})
}

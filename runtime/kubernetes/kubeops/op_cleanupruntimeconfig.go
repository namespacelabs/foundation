// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubeops

import (
	"context"
	"fmt"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func registerCleanup() {
	ops.RegisterFunc(func(ctx context.Context, env planning.Context, d *schema.SerializedInvocation, cleanup *kubedef.OpCleanupRuntimeConfig) (*ops.HandleResult, error) {
		return tasks.Return(ctx, tasks.Action("kubernetes.cleanup").HumanReadablef(d.Description), func(ctx context.Context) (*ops.HandleResult, error) {
			// Remove configmap runtime configs no longer being used.

			cluster, err := kubedef.InjectedKubeCluster(ctx)
			if err != nil {
				return nil, err
			}

			configs, err := cluster.Client().CoreV1().ConfigMaps(cleanup.Namespace).List(ctx, v1.ListOptions{
				LabelSelector: kubedef.SerializeSelector(map[string]string{
					kubedef.K8sKind: kubedef.K8sRuntimeConfigKind,
				}),
			})
			if err != nil {
				return nil, err
			}

			if len(configs.Items) == 0 {
				return nil, nil
			}

			usedConfigs := map[string]struct{}{}

			if cleanup.CheckPods {
				pods, err := cluster.Client().CoreV1().Pods(cleanup.Namespace).List(ctx, v1.ListOptions{
					LabelSelector: kubedef.SerializeSelector(kubedef.ManagedByUs()),
				})
				if err != nil {
					return nil, err
				}

				for _, d := range pods.Items {
					if v, ok := d.Annotations[kubedef.K8sRuntimeConfig]; ok {
						usedConfigs[v] = struct{}{}
					}
				}
			} else {
				deployments, err := cluster.Client().AppsV1().Deployments(cleanup.Namespace).List(ctx, v1.ListOptions{
					LabelSelector: kubedef.SerializeSelector(kubedef.ManagedByUs()),
				})
				if err != nil {
					return nil, err
				}

				for _, d := range deployments.Items {
					if v, ok := d.Annotations[kubedef.K8sRuntimeConfig]; ok {
						usedConfigs[v] = struct{}{}
					}
				}

				statefulSets, err := cluster.Client().AppsV1().StatefulSets(cleanup.Namespace).List(ctx, v1.ListOptions{
					LabelSelector: kubedef.SerializeSelector(kubedef.ManagedByUs()),
				})
				if err != nil {
					return nil, err
				}

				for _, d := range statefulSets.Items {
					if v, ok := d.Annotations[kubedef.K8sRuntimeConfig]; ok {
						usedConfigs[v] = struct{}{}
					}
				}
			}

			for _, cfg := range configs.Items {
				if _, ok := usedConfigs[cfg.Name]; ok {
					continue
				}

				if err := cluster.Client().CoreV1().ConfigMaps(cleanup.Namespace).Delete(ctx, cfg.Name, v1.DeleteOptions{}); err != nil {
					fmt.Fprintf(console.Warnings(ctx), "kubernetes: failed to remove unused runtime configuration %q: %v\n", cfg.Name, err)
				}
			}

			return nil, nil
		})
	})
}

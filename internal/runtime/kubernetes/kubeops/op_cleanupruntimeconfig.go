// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package kubeops

import (
	"context"
	"fmt"
	"time"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/framework/kubernetes/kubeobj"
	"namespacelabs.dev/foundation/internal/console"
	fnschema "namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/execution"
	"namespacelabs.dev/foundation/std/tasks"
)

func registerCleanup() {
	execution.RegisterFuncs(execution.Funcs[*kubedef.OpCleanupRuntimeConfig]{
		Handle: func(ctx context.Context, d *fnschema.SerializedInvocation, cleanup *kubedef.OpCleanupRuntimeConfig) (*execution.HandleResult, error) {
			return tasks.Return(ctx, tasks.Action("kubernetes.cleanup").HumanReadable(d.Description), func(ctx context.Context) (*execution.HandleResult, error) {
				// TODO turn into noop when orchestrator with corresponding controller is published.

				// Remove configmap runtime configs no longer being used.

				cluster, err := kubedef.InjectedKubeCluster(ctx)
				if err != nil {
					return nil, err
				}

				client := cluster.PreparedClient().Clientset

				configs, err := client.CoreV1().ConfigMaps(cleanup.Namespace).List(ctx, v1.ListOptions{
					LabelSelector: kubeobj.SerializeSelector(map[string]string{
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
					pods, err := client.CoreV1().Pods(cleanup.Namespace).List(ctx, v1.ListOptions{
						LabelSelector: kubeobj.SerializeSelector(kubedef.ManagedByUs()),
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
					deployments, err := client.AppsV1().Deployments(cleanup.Namespace).List(ctx, v1.ListOptions{
						LabelSelector: kubeobj.SerializeSelector(kubedef.ManagedByUs()),
					})
					if err != nil {
						return nil, err
					}

					for _, d := range deployments.Items {
						if v, ok := d.Annotations[kubedef.K8sRuntimeConfig]; ok {
							usedConfigs[v] = struct{}{}
						}
					}

					statefulSets, err := client.AppsV1().StatefulSets(cleanup.Namespace).List(ctx, v1.ListOptions{
						LabelSelector: kubeobj.SerializeSelector(kubedef.ManagedByUs()),
					})
					if err != nil {
						return nil, err
					}

					for _, d := range statefulSets.Items {
						if v, ok := d.Annotations[kubedef.K8sRuntimeConfig]; ok {
							usedConfigs[v] = struct{}{}
						}
					}

					daemonSets, err := client.AppsV1().DaemonSets(cleanup.Namespace).List(ctx, v1.ListOptions{
						LabelSelector: kubeobj.SerializeSelector(kubedef.ManagedByUs()),
					})
					if err != nil {
						return nil, err
					}

					for _, d := range daemonSets.Items {
						if v, ok := d.Annotations[kubedef.K8sRuntimeConfig]; ok {
							usedConfigs[v] = struct{}{}
						}
					}
				}

				for _, cfg := range configs.Items {
					if _, ok := usedConfigs[cfg.Name]; ok {
						continue
					}

					if time.Since(cfg.CreationTimestamp.Time) < time.Hour {
						continue
					}

					if err := client.CoreV1().ConfigMaps(cleanup.Namespace).Delete(ctx, cfg.Name, v1.DeleteOptions{}); err != nil {
						if !k8serrors.IsNotFound(err) {
							fmt.Fprintf(console.Warnings(ctx), "kubernetes: failed to remove unused runtime configuration %q: %v\n", cfg.Name, err)
						}
					}
				}

				return nil, nil
			})
		},

		PlanOrder: func(ctx context.Context, _ *kubedef.OpCleanupRuntimeConfig) (*fnschema.ScheduleOrder, error) {
			return &fnschema.ScheduleOrder{
				SchedAfterCategory: kubedef.Sched_JobLike,
			}, nil
		},
	})
}

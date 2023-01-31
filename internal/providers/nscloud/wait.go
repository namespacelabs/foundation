// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package nscloud

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/kubeobserver"
	"namespacelabs.dev/foundation/std/tasks"
)

const kubeSystem = "kube-system"

var deployments = map[string]struct{}{
	"coredns":                {},
	"local-path-provisioner": {},
}

func WaitKubeSystem(ctx context.Context, cluster *api.KubernetesCluster) error {
	return tasks.Action("cluster.wait-kube-system").
		Arg("id", cluster.ClusterId).Run(ctx, func(ctx context.Context) error {
		cfg := clientcmd.NewDefaultClientConfig(MakeConfig(cluster), nil)
		restcfg, err := cfg.ClientConfig()
		if err != nil {
			return fnerrors.New("failed to load kubernetes configuration: %w", err)
		}

		cli, err := k8s.NewForConfig(restcfg)
		if err != nil {
			return fnerrors.New("failed to create kubernetes client: %w", err)
		}

		var list *appsv1.DeploymentList
		// First wait until all critical deployments exist.
		if err := client.PollImmediateWithContext(ctx, 500*time.Millisecond, 5*time.Minute, func(ctx context.Context) (done bool, err error) {
			list, err = cli.AppsV1().Deployments(kubeSystem).List(ctx, metav1.ListOptions{})
			if err != nil {
				return false, err
			}

			names := map[string]struct{}{}
			for _, d := range list.Items {
				names[d.Name] = struct{}{}
			}

			for d := range deployments {
				if _, ok := names[d]; !ok {
					return false, nil
				}
			}

			return true, nil
		}); err != nil {
			return fnerrors.New("failed to wait for deployments in namespace %q: %w", kubeSystem, err)
		}

		eg := executor.New(ctx, "wait")

		for _, d := range list.Items {
			d := d

			if _, ok := deployments[d.Name]; !ok {
				fmt.Fprintf(console.Debug(ctx), "skipping non-critical kube-system deployment %s\n", d.Name)
				continue
			}

			eg.Go(func(ctx context.Context) error {
				fmt.Fprintf(console.Debug(ctx), "will wait for %s version %d\n", d.Name, d.Generation)

				obs := kubeobserver.WaitOnResource{
					RestConfig:       restcfg,
					Name:             d.Name,
					Namespace:        kubeSystem,
					Description:      fmt.Sprintf("kube-system deployment %s", d.Name),
					GroupVersionKind: schema.FromAPIVersionAndKind("apps/v1", "Deployment"),
					ExpectedGen:      d.Generation,
				}

				return obs.WaitUntilReady(ctx, nil)
			})
		}

		return eg.Wait()
	})

}

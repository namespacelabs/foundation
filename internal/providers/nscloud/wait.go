// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package nscloud

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/clientcmd"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/kubeobserver"
	"namespacelabs.dev/foundation/std/tasks"
)

const kubeSystem = "kube-system"

var deployments = []string{
	"coredns",
	"local-path-provisioner",
}

func WaitKubeSystem(ctx context.Context, cluster *api.KubernetesCluster) error {
	return tasks.Action("cluster.wait-kube-system").
		Arg("id", cluster.ClusterId).Run(ctx, func(ctx context.Context) error {
		cfg := clientcmd.NewDefaultClientConfig(MakeConfig(cluster), nil)
		restcfg, err := cfg.ClientConfig()
		if err != nil {
			return fnerrors.New("failed to load kubernetes configuration: %w", err)
		}

		eg := executor.New(ctx, "wait")

		for _, d := range deployments {
			d := d

			eg.Go(func(ctx context.Context) error {
				fmt.Fprintf(console.Debug(ctx), "will wait for deployment %s\n", d)

				obs := kubeobserver.WaitOnResource{
					RestConfig:       restcfg,
					Name:             d,
					Namespace:        kubeSystem,
					Description:      fmt.Sprintf("kube-system deployment %s", d),
					GroupVersionKind: schema.FromAPIVersionAndKind("apps/v1", "Deployment"),
				}

				return obs.WaitUntilReady(ctx, nil)
			})
		}

		return eg.Wait()
	})

}

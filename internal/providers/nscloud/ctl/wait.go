// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package ctl

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cenkalti/backoff/v4"
	"k8s.io/client-go/tools/clientcmd"
	"namespacelabs.dev/foundation/framework/kubernetes/kubeclient"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
	"namespacelabs.dev/foundation/std/tasks"
)

const (
	kubeSystem  = "kube-system"
	waitTimeout = 5 * time.Minute
	waitBackoff = 500 * time.Millisecond
)

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
			return fnerrors.Newf("failed to load kubernetes configuration: %w", err)
		}

		cli, err := kubeclient.NewREST(restcfg)
		if err != nil {
			return fnerrors.Newf("failed to create kubernetes client: %w", err)
		}

		eg := executor.New(ctx, "wait")

		for _, d := range deployments {
			eg.Go(func(ctx context.Context) error {
				fmt.Fprintf(console.Debug(ctx), "will wait for deployment %s\n", d)

				return waitForDeployment(ctx, cli, kubeSystem, d)
			})
		}

		return eg.Wait()
	})
}

// deploymentStatus is the subset of apps/v1 Deployment that readiness depends on.
type deploymentStatus struct {
	Status struct {
		Replicas        int32 `json:"replicas"`
		ReadyReplicas   int32 `json:"readyReplicas"`
		UpdatedReplicas int32 `json:"updatedReplicas"`
	} `json:"status"`
}

func waitForDeployment(ctx context.Context, cli *kubeclient.REST, namespace, name string) error {
	ctx, cancel := context.WithTimeout(ctx, waitTimeout)
	defer cancel()

	path := fmt.Sprintf("/apis/apps/v1/namespaces/%s/deployments/%s", namespace, name)

	return backoff.Retry(func() error {
		var dep deploymentStatus
		if err := cli.Get(ctx, path, &dep); err != nil {
			var status *kubeclient.StatusError
			if errors.As(err, &status) && status.IsNotFound() {
				// The deployment may not be visible yet; keep waiting.
				return fnerrors.Newf("deployment %s/%s not found yet", namespace, name)
			}

			return backoff.Permanent(err)
		}

		st := dep.Status
		if st.Replicas > 0 && st.ReadyReplicas == st.Replicas && st.UpdatedReplicas == st.Replicas {
			return nil
		}

		return fnerrors.Newf("deployment %s/%s not ready yet (ready=%d/%d, updated=%d)",
			namespace, name, st.ReadyReplicas, st.Replicas, st.UpdatedReplicas)
	}, backoff.WithContext(backoff.NewConstantBackOff(waitBackoff), ctx))
}

func WaitContainers(ctx context.Context, clusterId string, ctrs []*api.Container) error {
	return tasks.Action("cluster.wait-containers").HumanReadable("Waiting for containers to start").
		Arg("id", clusterId).Run(ctx, func(ctx context.Context) error {
		fmt.Fprintf(console.Debug(ctx), "polling cluster %q\n", clusterId)
		ctx, cancel := context.WithTimeout(ctx, waitTimeout)
		defer cancel()

		return backoff.Retry(func() error {
			res, err := api.GetClusterSummary(ctx, api.Methods, clusterId, []string{"nsc/containers"})
			if err != nil {
				return fmt.Errorf("failed to fetch cluster summary: %w", err)
			}

			resources := map[string]*api.Resource{} // keyed by UID
			for _, sum := range res.Summary {
				for _, r := range sum.PerResource {
					resources[r.Uid] = &r
				}
			}

			for _, ctr := range ctrs {
				r, ok := resources[ctr.Id]
				if !ok {
					return fmt.Errorf("no summary for requested container %q yet", ctr.Id)
				}

				for _, c := range r.Container {
					if !c.Ready {
						msg := fmt.Sprintf("container %q is not ready", c.Id)
						if c.NotRunningReason != "" {
							msg = fmt.Sprintf("%s: %s", msg, c.NotRunningReason)
						}

						return errors.New(msg)
					}
				}
			}

			return nil
		}, backoff.WithContext(backoff.NewConstantBackOff(waitBackoff), ctx))
	})
}

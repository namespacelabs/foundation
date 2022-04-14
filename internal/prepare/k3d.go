// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package prepare

import (
	"context"
	"fmt"

	"github.com/docker/docker/client"
	"namespacelabs.dev/foundation/build/registry"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/sdk/k3d"
	"namespacelabs.dev/foundation/runtime/docker"
	kubeclient "namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func PrepareK3d(clusterName string, env ops.Environment, updateKubecfg bool) compute.Computable[[]*schema.DevHost_ConfigureEnvironment] {
	return compute.Map(
		tasks.Action("prepare.k3d"),
		compute.Inputs(),
		compute.Output{},
		func(ctx context.Context, _ compute.Resolved) ([]*schema.DevHost_ConfigureEnvironment, error) {
			// download k3d
			k3dbin, err := k3d.EnsureSDK(ctx)
			if err != nil {
				return nil, err
			}

			cli, err := docker.NewClient()
			if err != nil {
				return nil, err
			}

			if err := k3d.ValidateDocker(ctx, cli); err != nil {
				return nil, err
			}

			var confs []*schema.DevHost_ConfigureEnvironment

			// XXX Need to support changing the registry of a cluster, for #340.
			// const registryName = "k3d-registry." + runtime.LocalBaseDomain
			const registryName = "k3d-registry.nslocal.dev"
			const registryPort = 41000
			if _, err := cli.ContainerInspect(ctx, registryName); err != nil {
				if !client.IsErrNotFound(err) {
					return nil, err
				}

				if err := k3dbin.CreateRegistry(ctx, registryName, registryPort); err != nil {
					return nil, err
				}
			}

			r := &registry.Registry{Url: fmt.Sprintf("http://%s:%d", registryName, registryPort)}

			c, err := devhost.MakeConfiguration(r)
			if err != nil {
				return nil, err
			}
			c.Purpose = schema.Environment_DEVELOPMENT
			confs = append(confs, c)

			clusters, err := k3dbin.ListClusters(ctx)
			if err != nil {
				return nil, err
			}

			var ours *k3d.Cluster
			for _, cl := range clusters {
				if cl.Name == clusterName {
					ours = &cl
				}
			}

			if ours == nil {
				// Create cluster.
				if err := k3dbin.CreateCluster(ctx, clusterName, fmt.Sprintf("%s:%d", registryName, registryPort), "rancher/k3s:v1.20.7-k3s1", updateKubecfg); err != nil {
					return nil, err
				}
			}

			if updateKubecfg {
				if err := k3dbin.MergeConfiguration(ctx, clusterName); err != nil {
					return nil, err
				}

				hostEnv := &kubeclient.HostEnv{
					Kubeconfig: "~/.kube/config",
					Context:    "k3d-" + clusterName,
				}

				c, err = devhost.MakeConfiguration(hostEnv)
				if err != nil {
					return nil, err
				}
				c.Purpose = env.Proto().GetPurpose()
				c.Runtime = "kubernetes"
				confs = append(confs, c)
			}

			return confs, nil
		})
}

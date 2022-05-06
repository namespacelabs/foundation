// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package prepare

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"namespacelabs.dev/foundation/build/registry"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/environment"
	"namespacelabs.dev/foundation/internal/sdk/k3d"
	"namespacelabs.dev/foundation/runtime/docker"
	"namespacelabs.dev/foundation/runtime/kubernetes"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func PrepareK3d(clusterName string, env ops.Environment) compute.Computable[*kubernetes.HostConfig] {
	return compute.Map(
		tasks.Action("prepare.k3d").HumanReadablef("Prepare the local k3d environment"),
		compute.Inputs().Str("clusterName", clusterName).Proto("env", env.Proto()),
		compute.Output{NotCacheable: true},
		func(ctx context.Context, _ compute.Resolved) (*kubernetes.HostConfig, error) {
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

			const registryName = "k3d-registry.nslocal.dev"
			registryCtr, err := cli.ContainerInspect(ctx, registryName)
			if err != nil {
				if !client.IsErrNotFound(err) {
					return nil, err
				}

				requestedRegistryPort := 41000
				// If running in CI, use dynamic port allocation to reduce probability of a port collision.
				// And in CI there's little need for stable addresses.
				if environment.IsRunningInCI() {
					requestedRegistryPort = 0
				}

				if err := k3dbin.CreateRegistry(ctx, registryName, requestedRegistryPort); err != nil {
					return nil, err
				}

				registryCtr, err = cli.ContainerInspect(ctx, registryName)
				if err != nil {
					return nil, err
				}
			}

			bindings := findPort(registryCtr, "5000/tcp")
			if len(bindings) == 0 {
				return nil, fmt.Errorf("registry: does not export port 5000 as expected")
			}

			registryPort := bindings[0].HostPort
			registryAddr := fmt.Sprintf("%s:%s", registryName, registryPort)

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
				if err := k3dbin.CreateCluster(ctx, clusterName, registryAddr, "rancher/k3s:v1.20.7-k3s1", true); err != nil {
					return nil, err
				}
			}

			if err := k3dbin.MergeConfiguration(ctx, clusterName); err != nil {
				return nil, err
			}

			r := &registry.Registry{Url: "http://" + registryAddr}
			return kubernetes.NewHostConfig("k3d-"+clusterName, env, kubernetes.WithRegistry(r))
		})
}

func findPort(ctr types.ContainerJSON, port string) []nat.PortBinding {
	if ctr.NetworkSettings == nil {
		return nil
	}

	return ctr.NetworkSettings.Ports[nat.Port(port)]
}

// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package prepare

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"namespacelabs.dev/foundation/framework/rpcerrors/multierr"
	"namespacelabs.dev/foundation/internal/build/registry"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/parsing/devhost"
	k3dp "namespacelabs.dev/foundation/internal/providers/k3d"
	"namespacelabs.dev/foundation/internal/runtime/docker"
	kubeclient "namespacelabs.dev/foundation/internal/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/internal/sdk/host"
	"namespacelabs.dev/foundation/internal/sdk/k3d"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/orchestration"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/tasks"
)

const registryName = "k3d-ns-registry.nslocal.host"

func K3D(clusterName string) Stage {
	return Stage{
		Pre: func(ch chan *orchestration.Event) {
			ch <- &orchestration.Event{
				Category:   "Local workstation",
				ResourceId: "docker-checker",
				Scope:      "Validate Docker version",
			}

			ch <- &orchestration.Event{
				Category:   "Local workstation",
				ResourceId: "k3s-cluster",
				Scope:      "Kubernetes cluster in Docker",
			}
		},

		Run: func(ctx context.Context, env cfg.Context, ch chan *orchestration.Event) (*schema.DevHost_ConfigureEnvironment, error) {
			return PrepareK3d(ctx, clusterName, env, ch)
		},
	}
}

func PrepareK3d(ctx context.Context, clusterName string, env cfg.Context, ch chan *orchestration.Event) (*schema.DevHost_ConfigureEnvironment, error) {
	return tasks.Return(ctx, tasks.Action("prepare.k3s").HumanReadablef("Preparing a local Kubernetes cluster (k3s running in Docker)"),
		func(ctx context.Context) (*schema.DevHost_ConfigureEnvironment, error) {
			dockerclient, err := docker.NewClient()
			if err != nil {
				return nil, err
			}

			if err := k3d.ValidateDocker(ctx, dockerclient); err != nil {
				return nil, err
			}

			ch <- &orchestration.Event{
				Category:   "Local workstation",
				ResourceId: "docker-checker",
				Ready:      orchestration.Event_READY,
				Stage:      orchestration.Event_DONE,
			}

			// download k3d
			k3dbin, err := k3d.EnsureSDK(ctx, host.HostPlatform())
			if err != nil {
				return nil, err
			}

			k3dPrepare := &k3dPrepare{clusterName, k3dbin, dockerclient}

			registryAddr, err := k3dPrepare.createOrRestartRegistry(ctx, registryName)
			if err != nil {
				return nil, err
			}
			if registryAddr == "" {
				return nil, fnerrors.InternalError("failed address registration for %q", registryName)
			}

			r := &k3dp.Configuration{
				RegistryContainerName: registryName,
				ClusterName:           clusterName,
			}

			hostEnv := kubeclient.NewLocalHostEnv("k3d-"+clusterName, env)
			c, err := devhost.MakeConfiguration(r, hostEnv, &registry.Provider{Provider: "k3d"})
			if err != nil {
				return nil, err
			}

			if err := k3dPrepare.createOrRestartCluster(ctx, clusterName, registryAddr); err != nil {
				return nil, err
			}

			ch <- &orchestration.Event{
				ResourceId: "k3s-cluster",
				Ready:      orchestration.Event_READY,
				Stage:      orchestration.Event_DONE,
			}

			return c, nil
		})
}

type k3dPrepare struct {
	clusterName  string
	k3dbin       k3d.K3D
	dockerclient docker.Client
}

func (p *k3dPrepare) createOrRestartRegistry(ctx context.Context, registryName string) (string, error) {
	registryCtr, err := p.dockerclient.ContainerInspect(ctx, registryName)
	if err != nil {
		if !client.IsErrNotFound(err) {
			return "", err
		}

		if err := p.k3dbin.CreateRegistry(ctx, registryName, 0); err != nil {
			return "", err
		}

		registryCtr, err = p.dockerclient.ContainerInspect(ctx, registryName)
		if err != nil {
			return "", err
		}
	}

	if !registryCtr.State.Running {
		if err := p.dockerclient.ContainerStart(ctx, registryName, types.ContainerStartOptions{}); err != nil {
			return "", fnerrors.InternalError("failed to restart registry %q: %w", registryName, err)
		}

		registryCtr, err = p.dockerclient.ContainerInspect(ctx, registryName)
		if err != nil {
			return "", fnerrors.InternalError("failed to inspect registry %q after a restart: %w", registryName, err)
		}
	}

	const expectedPort = "5000/tcp"

	registryPortBinding := findPort(registryCtr, expectedPort)
	if len(registryPortBinding) == 0 {
		return "", fnerrors.InternalError("failed to find expected port %q for registry %q", expectedPort, registryName)
	}

	registryPort := registryPortBinding[0].HostPort
	registryAddr := fmt.Sprintf("%s:%s", registryName, registryPort)
	return registryAddr, nil
}

func (p *k3dPrepare) createOrRestartCluster(ctx context.Context, clusterName string, registryAddr string) error {
	clusters, err := p.k3dbin.ListClusters(ctx)
	if err != nil {
		return err
	}

	var ours *k3d.Cluster
	for _, cl := range clusters {
		if cl.Name == clusterName {
			ours = &cl
		}
	}
	if ours == nil {
		// Create cluster.
		if err := p.k3dbin.CreateCluster(ctx, clusterName, registryAddr, "rancher/k3s:v1.22.9-k3s1", true); err != nil {
			return err
		}
	} else {
		// Ensure that all of the nodes in the cluster are up and running.
		var errs []error
		for _, node := range ours.Nodes {
			if !node.State.Running {
				if err := p.k3dbin.StartNode(ctx, node.Name); err != nil {
					errs = append(errs, err)
				}
			}
		}

		if err := multierr.New(errs...); err != nil {
			return fnerrors.InternalError("failed to start node(s) for cluster %q: %w", clusterName, err)
		}
	}

	if err := p.k3dbin.MergeConfiguration(ctx, clusterName); err != nil {
		return err
	}

	return nil
}

func findPort(ctr types.ContainerJSON, port string) []nat.PortBinding {
	if ctr.NetworkSettings == nil {
		return nil
	}

	return ctr.NetworkSettings.Ports[nat.Port(port)]
}

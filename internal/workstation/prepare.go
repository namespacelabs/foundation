// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package workstation

import (
	"context"
	"errors"
	"fmt"

	"github.com/docker/docker/client"
	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/types/known/anypb"
	corev1 "k8s.io/api/core/v1"
	"namespacelabs.dev/foundation/build/buildkit"
	"namespacelabs.dev/foundation/build/registry"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/sdk/k3d"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/docker"
	"namespacelabs.dev/foundation/runtime/kubernetes"
	kubeclient "namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/runtime/kubernetes/networking/ingress/nginx"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/foundation/workspace/tasks"
)

type PrepareResult struct {
	UpdateCount int
}

var deprecatedConfigs = []string{
	"type.googleapis.com/foundation.build.buildkit.Configuration",
}

func Prepare(ctx context.Context, root *workspace.Root, env ops.Environment, prepares ...PrepareFunc) (*PrepareResult, error) {
	var pr PrepareResult
	for _, prep := range prepares {
		confs, err := prep(ctx, env, &pr)
		if err != nil {
			return nil, err
		}

		if len(confs) == 0 {
			continue
		}

		updated, was := devhost.Update(root, confs...)
		if was {
			pr.UpdateCount++
		}

		// Make sure that the subsequent calls observe an up to date configuration.
		// XXX this is not right, Root() should be immutable.
		root.DevHost = updated
	}

	// Remove deprecated bits.
	for k, u := range root.DevHost.Configure {
		var without []*anypb.Any

		for _, cfg := range u.Configuration {
			if !slices.Contains(deprecatedConfigs, cfg.TypeUrl) {
				without = append(without, cfg)
			} else {
				pr.UpdateCount++
			}
		}

		if len(without) == 0 {
			root.DevHost.Configure[k] = nil // Mark for removal.
		} else {
			u.Configuration = without
		}
	}

	k := 0
	for {
		if k >= len(root.DevHost.Configure) {
			break
		}

		if root.DevHost.Configure[k] == nil {
			root.DevHost.Configure = append(root.DevHost.Configure[:k], root.DevHost.Configure[k+1:]...)
			pr.UpdateCount++
		} else {
			k++
		}
	}

	return &pr, nil
}

type PrepareFunc func(context.Context, ops.Environment, *PrepareResult) ([]*schema.DevHost_ConfigureEnvironment, error)

func PrepareBuildkit() PrepareFunc {
	return func(ctx context.Context, env ops.Environment, pr *PrepareResult) ([]*schema.DevHost_ConfigureEnvironment, error) {
		containerName := buildkit.DefaultContainerName

		conf := &buildkit.Overrides{}
		if !devhost.ConfigurationForEnv(env).Get(conf) {
			if conf.BuildkitAddr != "" {
				fmt.Fprintln(console.Stderr(ctx), "Buildkit has been manually configured, skipping setup.")
				return nil, nil
			}

			if conf.ContainerName != "" {
				containerName = conf.ContainerName
			}
		}

		_, err := buildkit.EnsureBuildkitd(ctx, containerName)
		return nil, err
	}
}

func PrepareK3d(clusterName string, updateKubecfg bool) PrepareFunc {
	return func(ctx context.Context, env ops.Environment, pr *PrepareResult) ([]*schema.DevHost_ConfigureEnvironment, error) {
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
	}
}

// XXX ideally setting up an ingress class would just be a dependent step in the graph; but
// we don't yet support waiting for these type of relationships.
func PrepareIngress(ctx context.Context, env ops.Environment, pr *PrepareResult) ([]*schema.DevHost_ConfigureEnvironment, error) {
	kube, err := kubernetes.New(env.Workspace(), env.DevHost(), env.Proto())
	if err != nil {
		return nil, err
	}

	state, err := kube.PrepareCluster(ctx)
	if err != nil {
		return nil, err
	}

	g := ops.NewRunner()
	if err := g.Add(state.Definitions()...); err != nil {
		return nil, err
	}

	waiters, err := g.Apply(ctx, runtime.TaskServerDeploy, noPackageEnv{env})
	if err != nil {
		return nil, err
	}

	if err := ops.WaitMultiple(ctx, waiters, nil); err != nil {
		return nil, err
	}

	// XXX this should be part of WaitUntilReady.
	if err := kube.Wait(ctx, tasks.Action("kubernetes.ingress.deploy"), kubernetes.WaitForPodConditition(
		kubernetes.SelectPods(nginx.IngressLoadBalancerService().Namespace, nil, nginx.ControllerSelector()),
		kubernetes.MatchPodCondition(corev1.PodReady))); err != nil {
		return nil, err
	}

	// The ingress produces no unique configuration.
	return nil, nil
}

type noPackageEnv struct {
	ops.Environment
}

var _ workspace.Packages = noPackageEnv{}

func (noPackageEnv) Resolve(ctx context.Context, packageName schema.PackageName) (workspace.Location, error) {
	return workspace.Location{}, errors.New("not supported")
}
func (noPackageEnv) LoadByName(ctx context.Context, packageName schema.PackageName) (*workspace.Package, error) {
	return nil, errors.New("not supported")
}

func SetK8sContext(name string) PrepareFunc {
	return func(ctx context.Context, env ops.Environment, pr *PrepareResult) ([]*schema.DevHost_ConfigureEnvironment, error) {
		hostEnv := &kubeclient.HostEnv{
			Kubeconfig: "~/.kube/config",
			Context:    name,
		}

		c, err := devhost.MakeConfiguration(hostEnv)
		if err != nil {
			return nil, err
		}
		c.Purpose = env.Proto().GetPurpose()
		c.Runtime = "kubernetes"
		return []*schema.DevHost_ConfigureEnvironment{c}, nil
	}
}

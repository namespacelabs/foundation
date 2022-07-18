// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package prepare

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubeobserver"
	"namespacelabs.dev/foundation/runtime/kubernetes/networking/ingress/nginx"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

type noPackageEnv struct {
	hostConfig *client.HostConfig
	ops.Environment
}

var _ workspace.Packages = noPackageEnv{}

func (noPackageEnv) Resolve(ctx context.Context, packageName schema.PackageName) (workspace.Location, error) {
	return workspace.Location{}, fnerrors.New("not supported")
}
func (noPackageEnv) LoadByName(ctx context.Context, packageName schema.PackageName) (*workspace.Package, error) {
	return nil, fnerrors.New("not supported")
}
func (noPackageEnv) Ensure(ctx context.Context, packageName schema.PackageName) error {
	return fnerrors.New("not supported")
}

func (p noPackageEnv) KubeconfigProvider() (*client.HostConfig, error) {
	return p.hostConfig, nil
}

func PrepareIngressFromHostConfig(env ops.Environment, k8sconfig compute.Computable[*client.HostConfig]) compute.Computable[[]*schema.DevHost_ConfigureEnvironment] {
	return PrepareIngress(env, compute.Transform(k8sconfig, func(ctx context.Context, cfg *client.HostConfig) (kubernetes.Unbound, error) {
		return kubernetes.NewFromConfig(ctx, cfg)
	}))
}

func PrepareIngress(env ops.Environment, k8sconfig compute.Computable[kubernetes.Unbound]) compute.Computable[[]*schema.DevHost_ConfigureEnvironment] {
	return compute.Map(
		tasks.Action("prepare.ingress").HumanReadablef("Deploying the Kubernetes ingress controller (may take up to 30 seconds)"),
		compute.Inputs().Computable("runtime", k8sconfig).Proto("env", env.Proto()).Proto("workspace", env.Workspace()),
		compute.Output{NotCacheable: true},
		func(ctx context.Context, deps compute.Resolved) ([]*schema.DevHost_ConfigureEnvironment, error) {
			kube := compute.MustGetDepValue(deps, k8sconfig, "runtime")

			if err := PrepareIngressInKube(ctx, env, kube); err != nil {
				return nil, err
			}

			// The ingress produces no unique configuration.
			return nil, nil
		})
}

func PrepareIngressInKube(ctx context.Context, env ops.Environment, kube kubernetes.Unbound) error {
	state, err := kube.PrepareCluster(ctx)
	if err != nil {
		return err
	}

	g := ops.NewPlan()
	if err := g.Add(state.Definitions()...); err != nil {
		return err
	}

	waiters, err := g.Execute(ctx, runtime.TaskServerDeploy, noPackageEnv{kube.HostConfig(), env})
	if err != nil {
		return err
	}

	if err := ops.WaitMultiple(ctx, waiters, nil); err != nil {
		return err
	}

	// XXX this should be part of WaitUntilReady.
	if err := waitForIngress(ctx, kube, tasks.Action("kubernetes.ingress.deploy")); err != nil {
		return err
	}

	return nil
}

func waitForIngress(ctx context.Context, kube kubernetes.Unbound, action *tasks.ActionEvent) error {
	return kubeobserver.WaitForCondition(ctx, kube.Client(), action, kubeobserver.WaitForPodConditition(
		kubeobserver.SelectPods(nginx.IngressLoadBalancerService().Namespace, nil, nginx.ControllerSelector()),
		kubeobserver.MatchPodCondition(corev1.PodReady)))
}

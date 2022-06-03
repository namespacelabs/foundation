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

func (p noPackageEnv) KubeconfigProvider() (*client.HostConfig, error) {
	return p.hostConfig, nil
}

func PrepareIngress(env ops.Environment, k8sconfig compute.Computable[*client.HostConfig]) compute.Computable[[]*schema.DevHost_ConfigureEnvironment] {
	return compute.Map(
		tasks.Action("prepare.ingress").HumanReadablef("Prepare and deploy the Kubernetes ingress controller"),
		compute.Inputs().Computable("k8sconfig", k8sconfig).Proto("env", env.Proto()),
		compute.Output{NotCacheable: true},
		func(ctx context.Context, deps compute.Resolved) ([]*schema.DevHost_ConfigureEnvironment, error) {
			config := compute.MustGetDepValue(deps, k8sconfig, "k8sconfig")

			kube, err := kubernetes.NewFromConfig(ctx, config)
			if err != nil {
				return nil, err
			}

			state, err := kube.PrepareCluster(ctx)
			if err != nil {
				return nil, err
			}

			g := ops.NewPlan()
			if err := g.Add(state.Definitions()...); err != nil {
				return nil, err
			}

			waiters, err := g.Execute(ctx, runtime.TaskServerDeploy, noPackageEnv{config, env})
			if err != nil {
				return nil, err
			}

			if err := ops.WaitMultiple(ctx, waiters, nil); err != nil {
				return nil, err
			}

			// XXX this should be part of WaitUntilReady.
			if err := waitForIngress(ctx, kube, tasks.Action("kubernetes.ingress.deploy")); err != nil {
				return nil, err
			}

			// The ingress produces no unique configuration.
			return nil, nil
		})
}

func waitForIngress(ctx context.Context, kube kubernetes.K8sRuntime, action *tasks.ActionEvent) error {
	return kube.Wait(ctx, action, kubeobserver.WaitForPodConditition(
		kubeobserver.SelectPods(nginx.IngressLoadBalancerService().Namespace, nil, nginx.ControllerSelector()),
		kubeobserver.MatchPodCondition(corev1.PodReady)))
}

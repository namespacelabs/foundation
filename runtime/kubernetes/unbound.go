// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"context"

	k8s "k8s.io/client-go/kubernetes"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubeobserver"
	"namespacelabs.dev/foundation/runtime/kubernetes/networking/ingress"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/tasks"
)

type Unbound struct {
	cli  *k8s.Clientset
	host *client.HostConfig
}

func NewFromConfig(ctx context.Context, config *client.HostConfig) (Unbound, error) {
	cli, err := client.NewClient(ctx, config)
	if err != nil {
		return Unbound{}, err
	}

	return Unbound{cli, config}, nil
}

func New(ctx context.Context, devHost *schema.DevHost, env *schema.Environment) (Unbound, error) {
	hostConfig, err := client.ComputeHostConfig(devHost, env)
	if err != nil {
		return Unbound{}, err
	}

	return NewFromConfig(ctx, hostConfig)
}

func (u Unbound) Bind(ws *schema.Workspace) K8sRuntime {
	return K8sRuntime{Unbound: u, moduleNamespace: moduleNamespace(ws, u.host.Env)}
}

func (r Unbound) PrepareCluster(ctx context.Context) (runtime.DeploymentState, error) {
	var state deploymentState

	ingressDefs, err := ingress.EnsureStack(ctx)
	if err != nil {
		return nil, err
	}

	state.definitions = ingressDefs

	return state, nil
}

func (r Unbound) Wait(ctx context.Context, action *tasks.ActionEvent, waiter kubeobserver.ConditionWaiter) error {
	return kubeobserver.WaitForCondition(ctx, r.cli, action, waiter)
}

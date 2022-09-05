// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"context"

	k8s "k8s.io/client-go/kubernetes"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/runtime/kubernetes/networking/ingress"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/planning"
)

type Unbound struct {
	cli            *k8s.Clientset
	computedClient *client.ComputedClient
	host           *client.HostConfig
}

func NewFromConfig(ctx context.Context, config *client.HostConfig) (Unbound, error) {
	cli, err := client.NewClient(ctx, config)
	if err != nil {
		return Unbound{}, err
	}

	return Unbound{cli.Clientset, cli, config}, nil
}

func NewFromEnv(ctx context.Context, env planning.Context) (Unbound, error) {
	return New(ctx, env.Configuration())
}

func New(ctx context.Context, cfg planning.Configuration) (Unbound, error) {
	hostConfig, err := client.ComputeHostConfig(cfg)
	if err != nil {
		return Unbound{}, err
	}

	return NewFromConfig(ctx, hostConfig)
}

func (u Unbound) Provider() (client.Provider, error) {
	return u.computedClient.Provider()
}

func (u Unbound) Client() *k8s.Clientset {
	return u.cli
}

func (u Unbound) HostConfig() *client.HostConfig {
	return u.host
}

func (u Unbound) Bind(ws *schema.Workspace, env *schema.Environment) K8sRuntime {
	return u.BindToNamespace(env, ModuleNamespace(ws, env))
}

func (u Unbound) BindToNamespace(env *schema.Environment, ns string) K8sRuntime {
	return K8sRuntime{Unbound: u, env: env, moduleNamespace: ns}
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

// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"context"

	k8s "k8s.io/client-go/kubernetes"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubetool"
	"namespacelabs.dev/foundation/runtime/kubernetes/networking/ingress"
	"namespacelabs.dev/foundation/std/planning"
)

type Cluster struct {
	cli            *k8s.Clientset
	computedClient *client.ComputedClient
	host           *client.HostConfig
}

func NewFromConfig(ctx context.Context, config *client.HostConfig) (Cluster, error) {
	cli, err := client.NewClient(ctx, config)
	if err != nil {
		return Cluster{}, err
	}

	return Cluster{cli.Clientset, cli, config}, nil
}

func New(ctx context.Context, cfg planning.Configuration) (Cluster, error) {
	hostConfig, err := client.ComputeHostConfig(cfg)
	if err != nil {
		return Cluster{}, err
	}

	return NewFromConfig(ctx, hostConfig)
}

func (u Cluster) Provider() (client.Provider, error) {
	return u.computedClient.Provider()
}

func (u Cluster) Client() *k8s.Clientset {
	return u.cli
}

func (u Cluster) HostConfig() *client.HostConfig {
	return u.host
}

func (u Cluster) Bind(env planning.Context) ClusterNamespace {
	ns := ModuleNamespace(env.Workspace().Proto(), env.Environment())

	conf := &kubetool.KubernetesEnv{}
	if env.Configuration().Get(conf) {
		ns = conf.Namespace
	}

	return ClusterNamespace{Cluster: u, clusterTarget: clusterTarget{env: env.Environment(), namespace: ns}}
}

func (r Cluster) PrepareCluster(ctx context.Context) (runtime.DeploymentState, error) {
	var state deploymentState

	ingressDefs, err := ingress.EnsureStack(ctx)
	if err != nil {
		return nil, err
	}

	state.definitions = ingressDefs

	return state, nil
}

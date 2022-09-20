// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"context"
	"sync"

	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/runtime/kubernetes/networking/ingress"
	"namespacelabs.dev/foundation/std/planning"
)

type Cluster struct {
	cli            *k8s.Clientset
	computedClient *client.ComputedClient
	host           *client.HostConfig

	ClusterAttachedState
}

type ClusterAttachedState struct {
	mu            sync.Mutex
	attachedState map[string]*state
}

type state struct {
	mu       sync.Mutex
	resolved bool
	value    any
	err      error
}

var _ kubedef.KubeCluster = &Cluster{}

func ConnectToConfig(ctx context.Context, config *client.HostConfig) (*Cluster, error) {
	cli, err := client.NewClient(ctx, config)
	if err != nil {
		return nil, err
	}

	return &Cluster{cli: cli.Clientset, computedClient: cli, host: config}, nil
}

func ConnectToCluster(ctx context.Context, cfg planning.Configuration) (*Cluster, error) {
	hostConfig, err := client.ComputeHostConfig(cfg)
	if err != nil {
		return nil, err
	}

	return ConnectToConfig(ctx, hostConfig)
}

func (u *Cluster) Class() runtime.Class {
	return kubernetesClass{}
}

func (u *Cluster) Planner(env planning.Context) runtime.Planner {
	return NewPlanner(env, u.SystemInfo)
}

func (u *Cluster) ClusterConfiguration() client.ClusterConfiguration {
	return u.computedClient.ClusterConfiguration()
}

func (u *Cluster) Client() *k8s.Clientset {
	return u.cli
}

func (u *Cluster) RESTConfig() *rest.Config {
	return u.computedClient.RESTConfig
}

func (u *Cluster) ComputedConfig() clientcmd.ClientConfig {
	return u.computedClient.ClientConfig()
}

func (u *Cluster) HostConfig() *client.HostConfig {
	return u.host
}

func (u *Cluster) Bind(env planning.Context) (runtime.ClusterNamespace, error) {
	return &ClusterNamespace{cluster: u, target: newTarget(env)}, nil
}

func (r *Cluster) PrepareCluster(ctx context.Context) (*runtime.DeploymentPlan, error) {
	var state runtime.DeploymentPlan

	ingressDefs, err := ingress.EnsureStack(ctx)
	if err != nil {
		return nil, err
	}

	state.Definitions = ingressDefs

	return &state, nil
}

func (r *Cluster) EnsureState(ctx context.Context, key string) (any, error) {
	return r.ClusterAttachedState.EnsureState(ctx, key, r.host.Config, r, nil)
}

func (r *ClusterAttachedState) EnsureState(ctx context.Context, stateKey string, config planning.Configuration, cluster runtime.Cluster, key *string) (any, error) {
	r.mu.Lock()
	if r.attachedState == nil {
		r.attachedState = map[string]*state{}
	}

	computedKey := stateKey
	if key != nil {
		computedKey += ":" + *key
	}

	if r.attachedState[computedKey] == nil {
		r.attachedState[computedKey] = &state{}
	}
	state := r.attachedState[computedKey]
	r.mu.Unlock()

	state.mu.Lock()
	defer state.mu.Unlock()

	if !state.resolved {
		if key != nil {
			state.value, state.err = runtime.PrepareKeyed(ctx, stateKey, config, cluster, *key)
		} else {
			state.value, state.err = runtime.Prepare(ctx, stateKey, config, cluster)
		}
		state.resolved = true
	}

	return state.value, state.err
}

// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"context"
	"sync"

	k8s "k8s.io/client-go/kubernetes"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/runtime/kubernetes/networking/ingress"
	"namespacelabs.dev/foundation/std/planning"
)

type Cluster struct {
	cli            *k8s.Clientset
	computedClient *client.ComputedClient
	host           *client.HostConfig

	mu            sync.Mutex
	attachedState map[string]*state
}

type state struct {
	mu       sync.Mutex
	resolved bool
	value    any
	err      error
}

func NewFromConfig(ctx context.Context, config *client.HostConfig) (*Cluster, error) {
	cli, err := client.NewClient(ctx, config)
	if err != nil {
		return nil, err
	}

	return &Cluster{cli: cli.Clientset, computedClient: cli, host: config}, nil
}

func NewCluster(ctx context.Context, cfg planning.Configuration) (*Cluster, error) {
	hostConfig, err := client.ComputeHostConfig(cfg)
	if err != nil {
		return nil, err
	}

	return NewFromConfig(ctx, hostConfig)
}

func NewNamespacedCluster(ctx context.Context, env planning.Context) (*ClusterNamespace, error) {
	cluster, err := NewCluster(ctx, env.Configuration())
	if err != nil {
		return nil, err
	}
	bound, err := cluster.Bind(env)
	if err != nil {
		return nil, err
	}
	return bound.(*ClusterNamespace), nil
}

func (u *Cluster) Class() runtime.Class {
	return kubernetesClass{}
}

func (u *Cluster) Planner(env planning.Context) runtime.Planner {
	return NewPlanner(env, u.SystemInfo)
}

func (u *Cluster) Provider() (client.ClusterConfiguration, error) {
	return u.computedClient.Provider()
}

func (u *Cluster) Client() *k8s.Clientset {
	return u.cli
}

func (u *Cluster) HostConfig() *client.HostConfig {
	return u.host
}

func (u *Cluster) Bind(env planning.Context) (runtime.ClusterNamespace, error) {
	return &ClusterNamespace{cluster: u, target: newTarget(env)}, nil
}

func (r *Cluster) PrepareCluster(ctx context.Context) (runtime.DeploymentState, error) {
	var state deploymentState

	ingressDefs, err := ingress.EnsureStack(ctx)
	if err != nil {
		return nil, err
	}

	state.definitions = ingressDefs

	return state, nil
}

func (r *Cluster) Prepare(ctx context.Context, key string, env planning.Context) (any, error) {
	r.mu.Lock()
	if r.attachedState == nil {
		r.attachedState = map[string]*state{}
	}
	if r.attachedState[key] == nil {
		r.attachedState[key] = &state{}
	}
	state := r.attachedState[key]
	r.mu.Unlock()

	state.mu.Lock()
	defer state.mu.Unlock()

	if !state.resolved {
		state.value, state.err = runtime.Prepare(ctx, key, env, r)
		state.resolved = true
	}

	return state.value, state.err
}

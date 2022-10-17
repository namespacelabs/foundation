// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"context"
	"sync"

	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"namespacelabs.dev/foundation/internal/tcache"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/runtime/kubernetes/networking/ingress"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/std/tasks"
)

type Cluster struct {
	cli            *k8s.Clientset
	computedClient client.Prepared
	config         planning.Configuration

	FetchSystemInfo func(context.Context) (*kubedef.SystemInfo, error)

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

func ConnectToCluster(ctx context.Context, config planning.Configuration) (*Cluster, error) {
	cli, err := client.NewClient(ctx, config)
	if err != nil {
		return nil, err
	}

	deferredSystemInfo := tcache.NewDeferred(tasks.Action("kubernetes.fetch-system-info"), func(ctx context.Context) (*kubedef.SystemInfo, error) {
		return computeSystemInfo(ctx, cli.Clientset)
	})

	return &Cluster{
		cli:             cli.Clientset,
		computedClient:  *cli,
		config:          config,
		FetchSystemInfo: deferredSystemInfo.Get,
	}, nil
}

func (u *Cluster) Class() runtime.Class {
	return kubernetesClass{}
}

func (u *Cluster) Planner(env planning.Context) runtime.Planner {
	return NewPlanner(env, u.SystemInfo)
}

func (u *Cluster) RESTConfig() *rest.Config {
	return u.computedClient.RESTConfig
}

func (u *Cluster) PreparedClient() client.Prepared {
	return u.computedClient
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
	return r.ClusterAttachedState.EnsureState(ctx, key, r.config, r, nil)
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

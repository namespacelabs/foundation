// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"namespacelabs.dev/foundation/build/registry"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	fnschema "namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/dirs"
)

type HostConfig struct {
	ws       *fnschema.Workspace
	devHost  *fnschema.DevHost
	env      *fnschema.Environment
	hostEnv  *client.HostEnv
	registry *registry.Registry
}

func NewHostConfig(contextName string, env ops.Environment, options ...func(*HostConfig)) (*HostConfig, error) {
	kubeconfig, err := dirs.ExpandHome("~/.kube/config")
	if err != nil {
		return nil, err
	}

	hostEnv := &client.HostEnv{
		Kubeconfig: kubeconfig,
		Context:    contextName,
	}

	config := &HostConfig{
		ws:      env.Workspace(),
		devHost: env.DevHost(),
		env:     env.Proto(),
		hostEnv: hostEnv,
	}

	for _, option := range options {
		option(config)
	}

	return config, nil
}

func WithRegistry(r *registry.Registry) func(*HostConfig) {
	return func(h *HostConfig) {
		h.registry = r
	}
}

func (h *HostConfig) Registry() *registry.Registry { return h.registry }

func (h *HostConfig) ClientHostEnv() *client.HostEnv { return h.hostEnv }

// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package client

import (
	"namespacelabs.dev/foundation/build/registry"
	"namespacelabs.dev/foundation/internal/engine/ops"
	fnschema "namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/foundation/workspace/dirs"
)

type HostConfig struct {
	DevHost  *fnschema.DevHost
	Selector devhost.Selector
	HostEnv  *HostEnv

	registry *registry.Registry
}

func NewHostConfig(contextName string, env ops.Environment, options ...func(*HostConfig)) (*HostConfig, error) {
	kubeconfig, err := dirs.ExpandHome("~/.kube/config")
	if err != nil {
		return nil, err
	}

	hostEnv := &HostEnv{
		Kubeconfig: kubeconfig,
		Context:    contextName,
	}

	config := &HostConfig{
		DevHost:  env.DevHost(),
		Selector: devhost.ByEnvironment(env.Proto()),
		HostEnv:  hostEnv,
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

func (h *HostConfig) ClientHostEnv() *HostEnv { return h.HostEnv }

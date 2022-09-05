// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package client

import (
	"namespacelabs.dev/foundation/build/registry"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace/dirs"
)

type HostConfig struct {
	Config  planning.Configuration
	HostEnv *HostEnv

	registry *registry.Registry
}

func NewHostConfig(contextName string, env planning.Context, options ...func(*HostConfig)) (*HostConfig, error) {
	kubeconfig, err := dirs.ExpandHome("~/.kube/config")
	if err != nil {
		return nil, err
	}

	hostEnv := &HostEnv{
		Kubeconfig: kubeconfig,
		Context:    contextName,
	}

	config := &HostConfig{
		Config:  env.Configuration(),
		HostEnv: hostEnv,
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

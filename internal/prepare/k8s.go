// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package prepare

import (
	"context"

	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/foundation/workspace/tasks"
)

type PrepareK8sOptions struct {
	contextName string
}

type PrepareK8sOption func(*PrepareK8sOptions)

func WithK8sContextName(contextName string) PrepareK8sOption {
	return func(options *PrepareK8sOptions) {
		options.contextName = contextName
	}
}

func PrepareExistingK8s(env ops.Environment, args ...PrepareK8sOption) compute.Computable[*client.HostConfig] {
	return compute.Map(
		tasks.Action("prepare.existing-k8s").HumanReadablef("Prepare a host-configured Kubernetes instance"),
		compute.Inputs().Proto("env", env.Proto()),
		compute.Output{NotCacheable: true},
		func(ctx context.Context, _ compute.Resolved) (*client.HostConfig, error) {
			opts := &PrepareK8sOptions{}
			for _, opt := range args {
				opt(opts)
			}
			if opts.contextName != "" {
				return client.NewHostConfig(opts.contextName, env)
			} else {
				return client.ComputeHostConfig(env.DevHost(), devhost.ByEnvironment(env.Proto()))
			}
		})
}

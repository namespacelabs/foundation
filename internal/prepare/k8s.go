// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package prepare

import (
	"context"

	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/runtime/kubernetes"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func PrepareExistingK8s(contextName string, env ops.Environment) compute.Computable[*kubernetes.HostConfig] {
	return compute.Map(
		tasks.Action("prepare.existing-k8s").HumanReadablef("Prepare a host-configured Kubernetes instance"),
		compute.Inputs().Str("contextName", contextName).Proto("env", env.Proto()),
		compute.Output{NotCacheable: true},
		func(ctx context.Context, _ compute.Resolved) (*kubernetes.HostConfig, error) {
			return kubernetes.NewHostConfig(contextName, env)
		})
}

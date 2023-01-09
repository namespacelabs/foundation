// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package prepare

import (
	"context"

	"namespacelabs.dev/foundation/framework/build/buildkit"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/internal/planning/deploy"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/execution"
	"namespacelabs.dev/foundation/std/execution/defs"
)

func PrepareBuildCluster(ctx context.Context, env cfg.Context, cluster *kubernetes.Cluster) (*buildkit.JobDefinition, error) {
	jobdef := buildkit.JobDef()

	var resources []kubedef.Apply

	resources = append(resources, kubedef.Apply{
		Description: "Buildkit Namespace",
		Resource:    jobdef.Namespace,
	})

	resources = append(resources, kubedef.Apply{
		Description: "Buildkit Server",
		Resource:    jobdef.StatefulSet,
	})

	resources = append(resources, kubedef.Apply{
		Description: "Buildkit Service",
		Resource:    jobdef.Service,
	})

	serialized, err := defs.Make(resources...)
	if err != nil {
		return nil, err
	}

	p := execution.NewPlan(serialized...)

	// Make sure that the cluster is accessible to a serialized invocation implementation.
	if err := execution.ExecuteExt(ctx, "deployment.execute", p,
		deploy.MaybeRenderClusterBlock(env, cluster, nil, true),
		execution.ExecuteOpts{WrapWithActions: true},
		execution.FromContext(env),
		runtime.ClusterInjection.With(cluster)); err != nil {
		return nil, err
	}

	return jobdef, nil
}

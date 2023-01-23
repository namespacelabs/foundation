// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package prepare

import (
	"context"

	corev1 "k8s.io/api/core/v1"
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

	platforms, err := cluster.UnmatchedTargetPlatforms(ctx)
	if err != nil {
		return nil, err
	}

	// Explicitly allow all pods on all available platforms.
	// On GKE, workloads are not allowed on ARM nodes by default, even if all nodes are ARM.
	// https://cloud.google.com/kubernetes-engine/docs/how-to/prepare-arm-workloads-for-deployment#overview
	// TODO make node affinity configurable.
	var archs []string
	for _, plat := range platforms {
		archs = append(archs, plat.Architecture)
	}

	jobdef.StatefulSet.Spec.Template.Spec.Affinity = &corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{{
					MatchExpressions: []corev1.NodeSelectorRequirement{{
						Key:      "kubernetes.io/arch",
						Operator: "In",
						Values:   archs,
					}},
				}},
			},
		},
	}

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

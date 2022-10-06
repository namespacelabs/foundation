// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubedef

import (
	"context"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/execution"
)

type KubeCluster interface {
	runtime.Cluster

	PreparedClient() client.Prepared
}

type KubeClusterNamespace interface {
	runtime.ClusterNamespace

	KubeConfig() KubeConfig
}

type KubeConfig struct {
	Context     string // Only set if explicitly set in KubeEnv.
	Namespace   string
	Environment *schema.Environment
}

func InjectedKubeCluster(ctx context.Context) (KubeCluster, error) {
	c, err := execution.Get(ctx, runtime.ClusterInjection)
	if err != nil {
		return nil, err
	}

	if v, ok := c.(KubeCluster); ok {
		return v, nil
	}

	return nil, fnerrors.InternalError("expected a kubernetes cluster in context")
}

func InjectedKubeClusterNamespace(ctx context.Context) (KubeClusterNamespace, error) {
	c, err := execution.Get(ctx, runtime.ClusterNamespaceInjection)
	if err != nil {
		return nil, err
	}

	if v, ok := c.(KubeClusterNamespace); ok {
		return v, nil
	}

	return nil, fnerrors.InternalError("expected a kubernetes namespace in context")
}

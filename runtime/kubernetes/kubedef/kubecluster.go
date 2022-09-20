// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubedef

import (
	"context"

	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"namespacelabs.dev/foundation/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/schema"
)

type KubeCluster interface {
	runtime.Cluster

	Client() *k8s.Clientset
	RESTConfig() *rest.Config
	ComputedConfig() clientcmd.ClientConfig
	ClusterConfiguration() client.ClusterConfiguration
}

type KubeClusterNamespace interface {
	runtime.ClusterNamespace

	KubeConfig() KubeConfig
}

type KubeConfig struct {
	Config, Context, Namespace string
	Environment                *schema.Environment
}

func InjectedKubeCluster(ctx context.Context) (KubeCluster, error) {
	c, err := ops.Get(ctx, runtime.ClusterInjection)
	if err != nil {
		return nil, err
	}

	if v, ok := c.(KubeCluster); ok {
		return v, nil
	}

	return nil, fnerrors.InternalError("expected a kubernetes cluster in context")
}

func InjectedKubeClusterNamespace(ctx context.Context) (KubeClusterNamespace, error) {
	c, err := ops.Get(ctx, runtime.ClusterNamespaceInjection)
	if err != nil {
		return nil, err
	}

	if v, ok := c.(KubeClusterNamespace); ok {
		return v, nil
	}

	return nil, fnerrors.InternalError("expected a kubernetes namespace in context")
}

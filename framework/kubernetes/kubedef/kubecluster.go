// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package kubedef

import (
	"context"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/orchestration"
	"namespacelabs.dev/foundation/std/execution"
)

type KubeCluster interface {
	runtime.Cluster

	Ingress() KubeIngress
	PreparedClient() client.Prepared
}

type KubeClusterNamespace interface {
	runtime.ClusterNamespace

	KubeConfig() KubeConfig
}

type KubeIngress interface {
	Service() *IngressSelector
	Waiter() KubeIngressWaiter
}

type KubeIngressWaiter interface {
	WaitUntilReady(ctx context.Context, ch chan *orchestration.Event) error
}

type IngressSelector struct {
	Namespace, ServiceName string
	ContainerPort          int
	PodSelector            map[string]string
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

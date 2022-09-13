// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubedef

import (
	"context"

	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime"
)

type KubeCluster interface {
	runtime.Cluster

	Client() *k8s.Clientset
	RESTConfig() *rest.Config
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

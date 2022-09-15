// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package ingress

import (
	"context"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/runtime/kubernetes/networking/ingress/nginx"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace/tasks"
)

const nginxIngressState = "ns.kubernetes.nginx"

func RegisterRuntimeState() {
	runtime.RegisterPrepare(nginxIngressState, func(ctx context.Context, cfg planning.Configuration, cluster runtime.Cluster) (any, error) {
		kube, ok := cluster.(kubedef.KubeCluster)
		if !ok {
			return nil, fnerrors.InternalError("%s: only supported with Kubernetes clusters", nginxIngressState)
		}

		if err := tasks.Action("ingress.wait").HumanReadablef("Waiting until nginx (ingress) is running").Run(ctx, func(ctx context.Context) error {
			return nginx.IngressWaiter(kube.RESTConfig()).WaitUntilReady(ctx, nil)
		}); err != nil {
			return nil, err
		}

		return nil, nil
	})
}

func EnsureState(ctx context.Context, cluster kubedef.KubeCluster) error {
	_, err := cluster.EnsureState(ctx, nginxIngressState)
	return err
}

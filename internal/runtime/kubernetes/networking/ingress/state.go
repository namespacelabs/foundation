// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package ingress

import (
	"context"

	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/networking/ingress/nginx"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/tasks"
)

const nginxIngressState = "ns.kubernetes.nginx"

func RegisterRuntimeState() {
	runtime.RegisterPrepare(nginxIngressState, func(ctx context.Context, cfg cfg.Configuration, cluster runtime.Cluster) (any, error) {
		kube, ok := cluster.(kubedef.KubeCluster)
		if !ok {
			return nil, fnerrors.InternalError("%s: only supported with Kubernetes clusters", nginxIngressState)
		}

		if err := tasks.Action("ingress.wait").HumanReadablef("Waiting until nginx (ingress) is running").Run(ctx, func(ctx context.Context) error {
			return nginx.IngressWaiter(kube.PreparedClient().RESTConfig).WaitUntilReady(ctx, nil)
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

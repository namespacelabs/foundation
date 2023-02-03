// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package kubernetes

import (
	"context"
	"fmt"
	"net"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/client"
)

type ServiceReadiness struct {
	Ready   bool
	Message string
}

func AreServicesReady(ctx context.Context, cli *kubernetes.Clientset, namespace string, srv runtime.Deployable) (ServiceReadiness, error) {
	if !client.IsInclusterClient(cli) {
		return ServiceReadiness{}, fnerrors.InternalError("cannot check service readiness for remote kubernetes cluster")
	}

	// TODO only check services that are required
	services, err := cli.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: kubedef.SerializeSelector(kubedef.SelectById(srv)),
	})
	if err != nil {
		return ServiceReadiness{}, err
	}

	for _, s := range services.Items {
		for _, port := range s.Spec.Ports {
			addr := fmt.Sprintf("%s.%s.svc.cluster.local:%d", s.Name, s.Namespace, port.Port)

			conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
			if err != nil {
				return ServiceReadiness{
					Ready:   false,
					Message: fmt.Sprintf("%q not ready: failed to dial %s:%d: %v", srv.GetName(), s.Name, port.Port, err),
				}, nil
			}
			conn.Close()
		}
	}

	return ServiceReadiness{Ready: true}, nil
}

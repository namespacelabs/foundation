// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package readiness

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"namespacelabs.dev/foundation/framework/kubernetes/kubeobj"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

type ServiceReadiness struct {
	Ready   bool
	Message string
}

type Deployable interface {
	GetId() string
	GetName() string
}

func isInclusterClient(c *kubernetes.Clientset) bool {
	config, err := rest.InClusterConfig()
	if err != nil {
		return false
	}

	u, err := url.Parse(config.Host)
	if err != nil {
		return false
	}

	return u.Host == c.RESTClient().Get().URL().Host
}

func AreServicesReady(ctx context.Context, cli *kubernetes.Clientset, namespace string, srv Deployable) (ServiceReadiness, error) {
	if !isInclusterClient(cli) {
		return ServiceReadiness{}, fnerrors.InternalError("cannot check service readiness for remote kubernetes cluster")
	}

	// TODO only check services that are required
	services, err := cli.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: kubeobj.SerializeSelector(kubeobj.SelectById(srv)),
	})
	if err != nil {
		return ServiceReadiness{}, err
	}

	for _, s := range services.Items {
		for _, port := range s.Spec.Ports {
			if port.Protocol != v1.ProtocolTCP {
				continue
			}

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

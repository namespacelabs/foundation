// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package service

import (
	"context"
	"fmt"

	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/networking/ingress"
	"namespacelabs.dev/foundation/internal/networking/ingress/nginx"
	"namespacelabs.dev/foundation/internal/planning/deploy"
	"namespacelabs.dev/foundation/internal/providers/gcp/gke"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/kubeops"
	"namespacelabs.dev/foundation/orchestration/proto"
	"namespacelabs.dev/foundation/std/go/server"
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/foundation/universe/aws/iam"
)

type Service struct {
}

func (svc *Service) AreServicesReady(ctx context.Context, req *proto.AreServicesReadyRequest) (*proto.AreServicesReadyResponse, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create incluster config: %w", err)
	}
	clientset, err := k8s.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create incluster clientset: %w", err)
	}

	res, err := kubernetes.AreServicesReady(ctx, clientset, req.Namespace, req.Deployable)
	if err != nil {
		return nil, err
	}

	return &proto.AreServicesReadyResponse{
		Ready:   res.Ready,
		Message: res.Message,
	}, nil
}

func WireService(ctx context.Context, srv server.Registrar, deps ServiceDeps) {
	proto.RegisterOrchestrationServiceServer(srv, &Service{})

	kubernetes.Register()
	kubeops.Register()
	iam.RegisterGraphHandlers()
	deploy.RegisterDeployOps()
	ingress.RegisterIngressClass(nginx.IngressClass())
	gke.RegisterIngressClass()

	// Always log actions, we filter if we show them on the client.
	tasks.LogActions = true
	// Always log debug to console, we redirect the log on the client.
	console.DebugToConsole = true
}

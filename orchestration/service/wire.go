// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package service

import (
	"context"
	"fmt"

	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/readiness"
	"namespacelabs.dev/foundation/orchestration/proto"
	"namespacelabs.dev/foundation/std/go/server"
)

type Service struct{}

func (svc *Service) AreServicesReady(ctx context.Context, req *proto.AreServicesReadyRequest) (*proto.AreServicesReadyResponse, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create incluster config: %w", err)
	}

	clientset, err := k8s.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create incluster clientset: %w", err)
	}

	res, err := readiness.AreServicesReady(ctx, clientset, req.Namespace, req.Deployable)
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
}

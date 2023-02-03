// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	pb "google.golang.org/protobuf/proto"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"namespacelabs.dev/foundation/framework/rpcerrors"
	"namespacelabs.dev/foundation/internal/auth"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/planning/deploy"
	"namespacelabs.dev/foundation/internal/providers/gcp/gke"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/kubeops"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/networking/ingress"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/networking/ingress/nginx"
	"namespacelabs.dev/foundation/orchestration"
	"namespacelabs.dev/foundation/orchestration/proto"
	"namespacelabs.dev/foundation/std/go/server"
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/foundation/universe/aws/iam"
)

type Service struct {
	deployer       *deployer
	versionChecker *versionChecker
}

func (svc *Service) Deploy(ctx context.Context, req *proto.DeployRequest) (*proto.DeployResponse, error) {
	log.Printf("new Deploy request for %d focus servers: %s\n", len(req.Plan.FocusServer), strings.Join(req.Plan.FocusServer, ","))
	now := time.Now()

	// XXX orchestrator should not write files; rather should inject authentication into session.
	if serialized := req.GetSerializedAuth(); serialized != nil {
		if err := auth.StoreMarshalledUser(ctx, req.GetSerializedAuth()); err != nil {
			return nil, err
		}
	} else if req.Auth != nil {
		data, err := json.Marshal(req.Auth)
		if err != nil {
			return nil, err
		}

		if err := auth.StoreMarshalledUser(ctx, data); err != nil {
			return nil, err
		}
	}

	var extra []pb.Message
	if req.Aws != nil {
		extra = append(extra, req.Aws)
	}

	env := orchestration.MakeSyntheticContext(req.Plan.Workspace, req.Plan.Environment, prepareHostEnv(req.HostEnv), extra...)

	// TODO store target state (req.Plan + merged with history) ?
	id, err := svc.deployer.Schedule(req.Plan, env, now)
	if err != nil {
		return nil, rpcerrors.Errorf(codes.Internal, "failed to deploy plan: %w", err)
	}

	res := &proto.DeployResponse{Id: id.ID}
	return res, nil
}

func (svc *Service) DeploymentStatus(req *proto.DeploymentStatusRequest, stream proto.OrchestrationService_DeploymentStatusServer) error {
	return svc.deployer.Status(stream.Context(), req.Id, req.LogLevel, stream.Send)
}

func (svc *Service) GetOrchestratorVersion(ctx context.Context, req *proto.GetOrchestratorVersionRequest) (*proto.GetOrchestratorVersionResponse, error) {
	return svc.versionChecker.GetOrchestratorVersion(req.SkipCache)
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

func prepareHostEnv(template *client.HostEnv) *client.HostEnv {
	res := template

	// Orchestrator runs in the same cluster as the deployment target
	// Pin incluster client and reset host-sepecific configuration
	res.Incluster = true

	res.Kubeconfig = ""
	res.Context = ""
	res.StaticConfig = nil

	return res
}

func WireService(ctx context.Context, srv server.Registrar, deps ServiceDeps) {
	proto.RegisterOrchestrationServiceServer(srv,
		&Service{
			deployer:       newDeployer(),
			versionChecker: newVersionChecker(ctx),
		})

	kubernetes.Register()
	kubeops.Register()
	iam.RegisterGraphHandlers()
	deploy.RegisterDeployOps()
	ingress.RegisterIngressClass(nginx.Ingress())
	gke.RegisterIngressClass()

	// Always log actions, we filter if we show them on the client.
	tasks.LogActions = true
	// Always log debug to console, we redirect the log on the client.
	console.DebugToConsole = true
}

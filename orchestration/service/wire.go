// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package service

import (
	"context"
	"log"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	pb "google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/framework/rpcerrors"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/planning/deploy"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/kubeops"
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

	if req.Auth != nil {
		if _, err := fnapi.StoreUser(ctx, req.Auth); err != nil {
			return nil, err
		}
	}

	var extra []pb.Message
	if req.Aws != nil {
		extra = append(extra, req.Aws)
	}

	env := orchestration.MakeSyntheticContext(req.Plan.Workspace, req.Plan.Environment, &client.HostEnv{Incluster: true}, extra...)

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

	// Always log actions, we filter if we show them on the client.
	tasks.LogActions = true
	// Always log debug to console, we redirect the log on the client.
	console.DebugToConsole = true
}

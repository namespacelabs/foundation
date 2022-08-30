// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package service

import (
	"context"
	"log"
	"os"
	"strings"

	"google.golang.org/grpc/codes"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/orchestration/service/proto"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubeops"
	"namespacelabs.dev/foundation/std/go/rpcerrors"
	"namespacelabs.dev/foundation/std/go/server"
	"namespacelabs.dev/foundation/workspace/tasks"
	"namespacelabs.dev/foundation/workspace/tasks/simplelog"
)

const maxLogLevel = 0

type Service struct {
}

func (svc *Service) Deploy(ctx context.Context, req *proto.DeployRequest) (*proto.DeployResponse, error) {
	log.Printf("new Deploy request for %d focus servers: %s\n", len(req.Plan.FocusServer), strings.Join(req.Plan.FocusServer, ","))

	// TODO store target state (req.Plan + merged with history)

	// TODO persist logs?
	sink := simplelog.NewSink(os.Stderr, maxLogLevel)
	ctxWithSink := tasks.WithSink(ctx, sink)

	env := makeEnv(req.Plan)
	p := ops.NewPlan()
	if err := p.Add(req.Plan.GetProgram().GetInvocation()...); err != nil {
		return nil, rpcerrors.Errorf(codes.Internal, "failed to prepare plan: %w", err)
	}

	waiters, err := p.Execute(ctxWithSink, runtime.TaskServerDeploy, env)
	if err != nil {
		return nil, err
	}

	// TODO create event channel and expose them via DeploymentStatus
	if err := ops.WaitMultiple(ctxWithSink, waiters, nil); err != nil {
		return nil, err
	}

	return &proto.DeployResponse{
		Id: "not-implemented-yet",
	}, nil
}
func (svc *Service) DeploymentStatus(req *proto.DeploymentStatusRequest, stream proto.OrchestrationService_DeploymentStatusServer) error {
	log.Printf("new DeploymentStatus request for deployment %s\n", req.Id)

	if err := stream.Send(&proto.DeploymentStatusResponse{
		Description: "Demo message",
	}); err != nil {
		return err
	}

	// Don't return error to not block CLI
	return nil
}

func WireService(ctx context.Context, srv server.Registrar, deps ServiceDeps) {
	proto.RegisterOrchestrationServiceServer(srv, &Service{})

	kubeops.Register()
}

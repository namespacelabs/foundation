// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package service

import (
	"context"
	"log"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/orchestration/proto"
	"namespacelabs.dev/foundation/providers/aws/iam"
	"namespacelabs.dev/foundation/runtime/kubernetes"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubeops"
	"namespacelabs.dev/foundation/schema/orchestration"
	"namespacelabs.dev/foundation/std/go/rpcerrors"
	"namespacelabs.dev/foundation/std/go/server"
	"namespacelabs.dev/foundation/workspace/tasks"
	"namespacelabs.dev/foundation/workspace/tasks/simplelog"
)

type Service struct {
	d deployer
}

func (svc *Service) Deploy(ctx context.Context, req *proto.DeployRequest) (*proto.DeployResponse, error) {
	log.Printf("new Deploy request for %d focus servers: %s\n", len(req.Plan.FocusServer), strings.Join(req.Plan.FocusServer, ","))
	now := time.Now()

	if req.Auth != nil {
		if _, err := fnapi.StoreUser(ctx, req.Auth); err != nil {
			return nil, err
		}
	}

	env := makeEnv(req.Plan, req.Aws)
	// TODO store target state (req.Plan + merged with history) ?
	id, err := svc.d.Schedule(req.Plan, env, now)
	if err != nil {
		return nil, rpcerrors.Errorf(codes.Internal, "failed to deploy plan: %w", err)
	}

	res := &proto.DeployResponse{Id: id}
	log.Printf("Will respond with %+v", res)
	return res, nil
}

func (svc *Service) DeploymentStatus(req *proto.DeploymentStatusRequest, stream proto.OrchestrationService_DeploymentStatusServer) error {
	log.Printf("new DeploymentStatus request for deployment %s\n", req.Id)

	errch := make(chan error, 1)
	ch := make(chan *orchestration.Event)

	go func() {
		defer close(errch)
		errch <- svc.d.Status(stream.Context(), req.Id, ch)
	}()

	for {
		ev, ok := <-ch
		if !ok {
			return <-errch
		}

		if err := stream.Send(&proto.DeploymentStatusResponse{
			Event: ev,
		}); err != nil {
			log.Printf("failed to send status response: %v", err)
		}
	}
}

func WireService(ctx context.Context, srv server.Registrar, deps ServiceDeps) {
	proto.RegisterOrchestrationServiceServer(srv, &Service{d: makeDeployer(ctx)})

	kubernetes.Register()
	kubeops.Register()
	iam.RegisterGraphHandlers()

	// Debug help - maybe delete later.
	tasks.LogActions = true
	simplelog.AlsoReportStartEvents = true
}

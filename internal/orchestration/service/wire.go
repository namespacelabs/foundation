// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package service

import (
	"context"
	"log"
	"strings"

	"google.golang.org/grpc/codes"
	"namespacelabs.dev/foundation/internal/orchestration/service/proto"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubeops"
	"namespacelabs.dev/foundation/std/go/rpcerrors"
	"namespacelabs.dev/foundation/std/go/server"
)

const maxLogLevel = 0

type Service struct {
	d deployer
}

func (svc *Service) Deploy(ctx context.Context, req *proto.DeployRequest) (*proto.DeployResponse, error) {
	log.Printf("new Deploy request for %d focus servers: %s\n", len(req.Plan.FocusServer), strings.Join(req.Plan.FocusServer, ","))

	// TODO store target state (req.Plan + merged with history) ?
	id, err := svc.d.Deploy(req.Plan)
	if err != nil {
		return nil, rpcerrors.Errorf(codes.Internal, "failed to deploy plan: %w", err)
	}

	return &proto.DeployResponse{
		Id: id,
	}, nil
}

func (svc *Service) DeploymentStatus(req *proto.DeploymentStatusRequest, stream proto.OrchestrationService_DeploymentStatusServer) error {
	log.Printf("new DeploymentStatus request for deployment %s\n", req.Id)

	s, err := svc.d.Status(req.Id)
	if err != nil {
		return err
	}

	for {
		ev, ok := <-s.events
		if !ok {
			// Event channel closed, check if there is an error
			err, ok := <-s.errch
			if ok {
				return err
			}
			return nil
		}

		if err := stream.Send(&proto.DeploymentStatusResponse{
			Event: ev,
		}); err != nil {
			return err
		}
	}
}

func WireService(ctx context.Context, srv server.Registrar, deps ServiceDeps) {
	proto.RegisterOrchestrationServiceServer(srv, &Service{
		d: deployer{serverCtx: ctx, m: make(map[string]*streams)},
	})

	kubeops.Register()
}

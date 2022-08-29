// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package service

import (
	"context"
	"log"
	"strings"

	"namespacelabs.dev/foundation/internal/orchestration/service/proto"
	"namespacelabs.dev/foundation/std/go/server"
)

type Service struct {
}

func (svc *Service) Deploy(ctx context.Context, req *proto.DeployRequest) (*proto.DeployResponse, error) {
	log.Printf("new Deploy request for %d focus servers: %s\n", len(req.Plan.FocusServer), strings.Join(req.Plan.FocusServer, ","))

	log.Printf("Deploy is not implemented")

	// Don't return error to not block CLI
	return &proto.DeployResponse{}, nil
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
}

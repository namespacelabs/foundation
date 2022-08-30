// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package service

import (
	"context"
	"log"
	"os"
	"strings"
	"sync"

	"google.golang.org/grpc/codes"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/orchestration/service/proto"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubeops"
	"namespacelabs.dev/foundation/schema/orchestration"
	"namespacelabs.dev/foundation/std/go/rpcerrors"
	"namespacelabs.dev/foundation/std/go/server"
	"namespacelabs.dev/foundation/workspace/tasks"
	"namespacelabs.dev/foundation/workspace/tasks/simplelog"
	"namespacelabs.dev/go-ids"
)

const maxLogLevel = 0

type Service struct {
	// TODO What if there are multiple readers?
	m  map[string]streams
	mu sync.Mutex
}

type streams struct {
	events chan *orchestration.Event
	errch  chan error
}

func (svc *Service) Deploy(ctx context.Context, req *proto.DeployRequest) (*proto.DeployResponse, error) {
	log.Printf("new Deploy request for %d focus servers: %s\n", len(req.Plan.FocusServer), strings.Join(req.Plan.FocusServer, ","))

	// TODO store target state (req.Plan + merged with history) ?

	env := makeEnv(req.Plan)
	p := ops.NewPlan()
	if err := p.Add(req.Plan.GetProgram().GetInvocation()...); err != nil {
		return nil, rpcerrors.Errorf(codes.Internal, "failed to prepare plan: %w", err)
	}

	id := ids.NewRandomBase32ID(16)
	errch := make(chan error, 1)
	ch := make(chan *orchestration.Event)

	svc.mu.Lock()
	svc.m[id] = streams{
		events: ch,
		errch:  errch,
	}
	svc.mu.Unlock()

	go func() {
		defer close(ch)
		defer close(errch)

		// TODO persist logs?
		sink := simplelog.NewSink(os.Stderr, maxLogLevel)
		ctxWithSink := tasks.WithSink(ctx, sink)

		waiters, err := p.Execute(ctxWithSink, runtime.TaskServerDeploy, env)
		if err != nil {
			errch <- err
			return
		}

		// TODO add observeContainers on channel
		if err := ops.WaitMultiple(ctxWithSink, waiters, ch); err != nil {
			errch <- err
			return
		}
	}()

	return &proto.DeployResponse{
		Id: id,
	}, nil
}

func (svc *Service) DeploymentStatus(req *proto.DeploymentStatusRequest, stream proto.OrchestrationService_DeploymentStatusServer) error {
	log.Printf("new DeploymentStatus request for deployment %s\n", req.Id)

	svc.mu.Lock()
	s, ok := svc.m[req.Id]
	svc.mu.Unlock()

	if !ok {
		return rpcerrors.Errorf(codes.InvalidArgument, "unknown deployment id: %s", req.Id)
	}

	for {
		ev, ok := <-s.events
		if !ok {
			// Event channel closed, check if there is an error and gc map.
			defer func() {
				svc.mu.Lock()
				defer svc.mu.Lock()
				delete(svc.m, req.Id)
			}()

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
		m: make(map[string]streams),
	})

	kubeops.Register()
}

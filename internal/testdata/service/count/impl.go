// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package count

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/types/known/emptypb"
	"namespacelabs.dev/foundation/std/go/server"
	"namespacelabs.dev/foundation/internal/testdata/counter"
	"namespacelabs.dev/foundation/internal/testdata/service/proto"
)

type Service struct {
	counters []counter.Counter
}

func (svc *Service) Increment(ctx context.Context, req *proto.IncrementRequest) (*emptypb.Empty, error) {
	for _, c := range svc.counters {
		if c.GetName() == req.Name {
			c.Increment()
			return &emptypb.Empty{}, nil
		}
	}

	return nil, fmt.Errorf("unknown counter %s", req.Name)
}

func (svc *Service) Get(ctx context.Context, req *proto.GetRequest) (*proto.GetResponse, error) {
	for _, c := range svc.counters {
		if c.GetName() == req.Name {
			return &proto.GetResponse{Value: c.Get()}, nil
		}
	}

	return nil, fmt.Errorf("unknown counter %s", req.Name)
}

func WireService(ctx context.Context, srv server.Registrar, deps ServiceDeps) {
	svc := &Service{counters: []counter.Counter{*deps.One, *deps.Two}}
	proto.RegisterCountServiceServer(srv, svc)
}

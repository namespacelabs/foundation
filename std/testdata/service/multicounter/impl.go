// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package multicounter

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/types/known/emptypb"
	"namespacelabs.dev/foundation/std/go/server"
	"namespacelabs.dev/foundation/std/testdata/counter"
)

type Service struct {
	counters []counter.Counter
}

func (svc *Service) Increment(ctx context.Context, req *IncrementRequest) (*emptypb.Empty, error) {
	for _, c := range svc.counters {
		if c.GetName() == req.Name {
			c.Increment()
			return &emptypb.Empty{}, nil
		}
	}

	return nil, fmt.Errorf("unknown counter %s", req.Name)
}

func (svc *Service) Get(ctx context.Context, req *GetRequest) (*GetResponse, error) {
	for _, c := range svc.counters {
		if c.GetName() == req.Name {
			return &GetResponse{Value: c.Get()}, nil
		}
	}

	return nil, fmt.Errorf("unknown counter %s", req.Name)
}

func WireService(ctx context.Context, srv server.Registrar, deps ServiceDeps) {
	svc := &Service{counters: []counter.Counter{*deps.One, *deps.Two}}
	RegisterMulticounterServiceServer(srv, svc)
}

// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package modeling

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"
	"namespacelabs.dev/foundation/std/go/grpc/server"
	"namespacelabs.dev/foundation/std/testdata/scopes"
)

type Service struct {
	deps *ServiceDeps
}

func (svc *Service) GetScopedData(ctx context.Context, _ *emptypb.Empty) (*GetScopedDataResponse, error) {
	response := &GetScopedDataResponse{Item: []*scopes.ScopedData{
		svc.deps.One,
		svc.deps.Two,
	}}

	return response, nil
}

func WireService(ctx context.Context, srv *server.Grpc, deps *ServiceDeps) {
	svc := &Service{deps: deps}
	RegisterModelingServiceServer(srv, svc)
}

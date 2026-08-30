// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fetch

import (
	"context"
	"errors"

	"namespacelabs.dev/foundation/std/go/server"
	"namespacelabs.dev/foundation/internal/testdata/service/proto"
)

type Service struct {
	proto.UnimplementedPostServiceServer
}

func (svc *Service) Fetch(ctx context.Context, req *proto.FetchRequest) (*proto.FetchResponse, error) {
	return nil, errors.New("come back and implement me")
}

func WireService(ctx context.Context, srv server.Registrar, deps ServiceDeps) {
	svc := &Service{}
	proto.RegisterPostServiceServer(srv, svc)
}

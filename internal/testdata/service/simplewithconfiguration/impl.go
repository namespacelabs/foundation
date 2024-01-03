// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package simplewithconfiguration

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"namespacelabs.dev/foundation/internal/testdata/service/proto"
	gogrpc "namespacelabs.dev/foundation/std/go/grpc"
	"namespacelabs.dev/foundation/std/go/server"
)

func init() {
	gogrpc.SetServiceConfiguration("mtls", conf{})
}

type Service struct {
}

func WireService(ctx context.Context, srv server.Registrar, deps ServiceDeps) {
	proto.RegisterEmptyServiceServer(srv, &Service{})
}

type conf struct{}

func (conf) TransportCredentials() credentials.TransportCredentials {
	return nil
}

func (conf) ServerOpts() []grpc.ServerOption { return nil }

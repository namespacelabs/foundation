// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package telemetryinfo

import (
	"context"

	"namespacelabs.dev/foundation/std/go/server"
)

func WireService(ctx context.Context, srv server.Registrar, deps ServiceDeps) {
	RegisterTelemetryInfoServiceServer(srv, &service{deps: deps})
}

type service struct {
	deps ServiceDeps
	UnimplementedTelemetryInfoServiceServer
}

func (s *service) GetServiceName(ctx context.Context, req *GetServiceNameRequest) (*GetServiceNameResponse, error) {
	resp := &GetServiceNameResponse{
		ServerName: s.deps.ServerInfo.ServerName,
	}

	if s.deps.ServerInfo.TelemetryResource != nil {
		resp.TelemetryServiceName = s.deps.ServerInfo.TelemetryResource.ServiceName
	}

	return resp, nil
}

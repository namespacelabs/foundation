// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"log"

	"google.golang.org/grpc"
	"namespacelabs.dev/foundation/internal/uniquestrings"
	"namespacelabs.dev/foundation/provision/configure"
	"namespacelabs.dev/foundation/provision/tool/protocol"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/allocations"
	"namespacelabs.dev/foundation/std/web/http"
)

func main() {
	if err := configure.RunServer(context.Background(), func(sr grpc.ServiceRegistrar) {
		protocol.RegisterPrepareServiceServer(sr, prepareHook{})
	}); err != nil {
		log.Fatal(err)
	}
}

type prepareHook struct{}

func (prepareHook) Prepare(ctx context.Context, req *protocol.PrepareRequest) (*protocol.PrepareResponse, error) {
	var serverList uniquestrings.List

	if err := allocations.Visit(req.Server.Allocation, "namespacelabs.dev/foundation/std/web/http", &http.Backend{},
		func(_ *schema.Allocation_Instance, _ *schema.Instantiate, backend *http.Backend) error {
			serverList.Add(backend.EndpointOwner)
			return nil
		}); err != nil {
		return nil, err
	}

	return &protocol.PrepareResponse{
		PreparedProvisionPlan: &protocol.PreparedProvisionPlan{
			DeclaredStack: serverList.Strings(),
		},
	}, nil
}

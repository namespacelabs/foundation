// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

// This file was automatically generated.
package grpcservice

import (
	"context"

	"namespacelabs.dev/foundation/std/go/grpc/server"
	"namespacelabs.dev/foundation/std/testdata/go/datastore"
)

type ServiceDeps struct {
	Main *datastore.DB
}

// Verify that WireService is present and has the appropriate type.
type checkWireService func(context.Context, *server.Grpc, ServiceDeps)

var _ checkWireService = WireService
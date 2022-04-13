// This file was automatically generated.
package post

import (
	"context"

	"google.golang.org/grpc"
	"namespacelabs.dev/foundation/std/go/grpc/server"
	"namespacelabs.dev/foundation/std/grpc/deadlines"
	"namespacelabs.dev/foundation/std/testdata/datastore"
	"namespacelabs.dev/foundation/std/testdata/service/simple"
)

// Dependencies that are instantiated once for the lifetime of the service.
type ServiceDeps struct {
	Dl         *deadlines.DeadlineRegistration
	Main       *datastore.DB
	Simple     simple.EmptyServiceClient
	SimpleConn *grpc.ClientConn
}

// Verify that WireService is present and has the appropriate type.
type checkWireService func(context.Context, *server.Grpc, ServiceDeps)

var _ checkWireService = WireService

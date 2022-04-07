// This file was automatically generated.
package post

import (
	"context"

	"namespacelabs.dev/foundation/std/go/grpc/server"
	"namespacelabs.dev/foundation/std/grpc/deadlines"
	"namespacelabs.dev/foundation/std/testdata/datastore"
)

type ServiceDeps struct {
	Dl   *deadlines.DeadlineRegistration
	Main *datastore.DB
}

// Verify that WireService is present and has the appropriate type.
type checkWireService func(context.Context, *server.Grpc, ServiceDeps)

var _ checkWireService = WireService

// This file was automatically generated.
package simple

import (
	"context"

	"namespacelabs.dev/foundation/std/go/grpc/server"
)

// Dependencies that are instantiated once for the lifetime of the extension
type ServiceDeps struct {
}

// Verify that WireService is present and has the appropriate type.
type checkWireService func(context.Context, *server.Grpc, *ServiceDeps)

var _ checkWireService = WireService

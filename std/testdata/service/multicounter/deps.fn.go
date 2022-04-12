// This file was automatically generated.
package multicounter

import (
	"context"

	"namespacelabs.dev/foundation/std/go/grpc/server"
	"namespacelabs.dev/foundation/std/testdata/counter"
)

// Dependencies that are instantiated once for the lifetime of the extension
type ServiceDeps struct {
	One *counter.Counter
	Two *counter.Counter
}

// Verify that WireService is present and has the appropriate type.
type checkWireService func(context.Context, *server.Grpc, ServiceDeps)

var _ checkWireService = WireService

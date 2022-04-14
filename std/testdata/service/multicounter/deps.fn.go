// This file was automatically generated.
package multicounter

import (
	"context"

	"namespacelabs.dev/foundation/std/go/server"
	"namespacelabs.dev/foundation/std/testdata/counter"
)

// Dependencies that are instantiated once for the lifetime of the service.
type ServiceDeps struct {
	One *counter.Counter
	Two *counter.Counter
}

// Verify that WireService is present and has the appropriate type.
type checkWireService func(context.Context, server.Registrar, ServiceDeps)

var _ checkWireService = WireService

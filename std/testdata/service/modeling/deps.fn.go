// This file was automatically generated.
package modeling

import (
	"context"

	"namespacelabs.dev/foundation/std/go/grpc/server"
	"namespacelabs.dev/foundation/std/testdata/scopes"
)

// Dependencies that are instantiated once for the lifetime of the extension
type ServiceDeps struct {
	One *scopes.ScopedData
	Two *scopes.ScopedData
}

// Verify that WireService is present and has the appropriate type.
type checkWireService func(context.Context, *server.Grpc, *ServiceDeps)

var _ checkWireService = WireService

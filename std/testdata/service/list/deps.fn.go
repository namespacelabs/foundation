// This file was automatically generated.
package list

import (
	"context"

	"github.com/jackc/pgx/v4/pgxpool"
	"namespacelabs.dev/foundation/std/go/server"
)

// Dependencies that are instantiated once for the lifetime of the service.
type ServiceDeps struct {
	Db *pgxpool.Pool
}

// Verify that WireService is present and has the appropriate type.
type checkWireService func(context.Context, server.Registrar, ServiceDeps)

var _ checkWireService = WireService

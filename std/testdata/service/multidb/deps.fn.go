// This file was automatically generated.
package multidb

import (
	"context"

	"github.com/jackc/pgx/v4/pgxpool"
	"namespacelabs.dev/foundation/std/go/grpc/server"
)

type ServiceDeps struct {
	Maria    *pgxpool.Pool
	Postgres *pgxpool.Pool
}

// Verify that WireService is present and has the appropriate type.
type checkWireService func(context.Context, *server.Grpc, ServiceDeps)

var _ checkWireService = WireService

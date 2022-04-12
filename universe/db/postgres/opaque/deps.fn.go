// This file was automatically generated.
package opaque

import (
	"context"

	"github.com/jackc/pgx/v4/pgxpool"
	"namespacelabs.dev/foundation/std/go/core"
	"namespacelabs.dev/foundation/universe/db/postgres/opaque/creds"
)

// Dependencies that are instantiated once for the lifetime of the extension
type ExtensionDeps struct {
	ReadinessCheck core.Check
}

// Scoped dependencies that are reinstantiated for each call to ProvideDatabase
type DatabaseDeps struct {
	Creds *creds.Creds
}

type _checkProvideDatabase func(context.Context, *Database, ExtensionDeps, DatabaseDeps) (*pgxpool.Pool, error)

var _ _checkProvideDatabase = ProvideDatabase

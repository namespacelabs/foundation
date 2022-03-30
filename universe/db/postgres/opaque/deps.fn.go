// This file was automatically generated.
package opaque

import (
	"context"

	"github.com/jackc/pgx/v4/pgxpool"
	"namespacelabs.dev/foundation/std/go/core"
	"namespacelabs.dev/foundation/universe/db/postgres/creds"
)

type ExtensionDeps struct {
	Creds          *creds.Creds
	ReadinessCheck core.Check
}

type _checkProvideDatabase func(context.Context, string, *Database, ExtensionDeps) (*pgxpool.Pool, error)

var _ _checkProvideDatabase = ProvideDatabase

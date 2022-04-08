// This file was automatically generated.
package opaque

import (
	"context"

	"github.com/jackc/pgx/v4/pgxpool"
	"namespacelabs.dev/foundation/std/go/core"
	fninit "namespacelabs.dev/foundation/std/go/core/init"
	"namespacelabs.dev/foundation/universe/db/postgres/opaque/creds"
)

type SingletonDeps struct {
	ReadinessCheck core.Check
}

// Scoped dependencies that are reinstantiated for each call to ProvideDatabase
type DatabaseDeps struct {
	Creds *creds.Creds
}

type _checkProvideDatabase func(context.Context, fninit.Caller, *Database, *SingletonDeps, *DatabaseDeps) (*pgxpool.Pool, error)

var _ _checkProvideDatabase = ProvideDatabase

// This file was automatically generated.
package incluster

import (
	"context"

	"github.com/jackc/pgx/v4/pgxpool"
	"namespacelabs.dev/foundation/std/go/core"
	"namespacelabs.dev/foundation/universe/db/postgres/incluster/creds"
)

// Dependencies that are instantiated once for the lifetime of the extension.
type ExtensionDeps struct {
	Creds          *creds.Creds
	ReadinessCheck core.Check
}

type _checkProvideDatabase func(context.Context, *Database, ExtensionDeps) (*pgxpool.Pool, error)

var _ _checkProvideDatabase = ProvideDatabase

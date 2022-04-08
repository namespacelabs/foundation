// This file was automatically generated.
package incluster

import (
	"context"

	"github.com/jackc/pgx/v4/pgxpool"
	"namespacelabs.dev/foundation/std/go/core"
	fninit "namespacelabs.dev/foundation/std/go/core/init"
	"namespacelabs.dev/foundation/universe/db/postgres/incluster/creds"
)

type SingletonDeps struct {
	Creds          *creds.Creds
	ReadinessCheck core.Check
}

type _checkProvideDatabase func(context.Context, fninit.Caller, *Database, *SingletonDeps) (*pgxpool.Pool, error)

var _ _checkProvideDatabase = ProvideDatabase

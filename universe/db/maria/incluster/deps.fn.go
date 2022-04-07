// This file was automatically generated.
package incluster

import (
	"context"

	"database/sql"
	"namespacelabs.dev/foundation/std/go/core"
	"namespacelabs.dev/foundation/universe/db/maria/incluster/creds"
)

type SingletonDeps struct {
	Creds          *creds.Creds
	ReadinessCheck core.Check
}

type _checkProvideDatabase func(context.Context, string, *Database, SingletonDeps) (*sql.DB, error)

var _ _checkProvideDatabase = ProvideDatabase

// This file was automatically generated.
package incluster

import (
	"context"

	"database/sql"
	"namespacelabs.dev/foundation/std/go/core"
	"namespacelabs.dev/foundation/universe/db/maria/incluster/creds"
)

type ExtensionDeps struct {
	Creds          *creds.Creds
	ReadinessCheck core.Check
}

type _checkProvideDatabase func(context.Context, string, *Database, ExtensionDeps) (*sql.DB, error)

var _ _checkProvideDatabase = ProvideDatabase

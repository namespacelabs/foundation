// This file was automatically generated.
package incluster

import (
	"context"

	"database/sql"
	"namespacelabs.dev/foundation/std/go/core"
	"namespacelabs.dev/foundation/universe/db/maria/incluster/creds"
)

// Dependencies that are instantiated once for the lifetime of the extension
type ExtensionDeps struct {
	Creds          *creds.Creds
	ReadinessCheck core.Check
}

type _checkProvideDatabase func(context.Context, *Database, *ExtensionDeps) (*sql.DB, error)

var _ _checkProvideDatabase = ProvideDatabase

// This file was automatically generated.
package datastore

import (
	"context"

	"namespacelabs.dev/foundation/std/go/core"
	"namespacelabs.dev/foundation/std/secrets"
)

type ExtensionDeps struct {
	Cert           *secrets.Value
	Gen            *secrets.Value
	ReadinessCheck core.Check
}

type _checkProvideDatabase func(context.Context, string, *Database, ExtensionDeps) (*DB, error)

var _ _checkProvideDatabase = ProvideDatabase

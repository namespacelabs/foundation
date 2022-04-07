// This file was automatically generated.
package datastore

import (
	"context"

	"namespacelabs.dev/foundation/std/go/core"
	"namespacelabs.dev/foundation/std/secrets"
)

type SingletonDeps struct {
	ReadinessCheck core.Check
}

// Scoped dependencies that are reinstantiated for each call to ProvideDatabase
type DatabaseDeps struct {
	Cert   *secrets.Value
	Gen    *secrets.Value
	Keygen *secrets.Value
}

type _checkProvideDatabase func(context.Context, string, *Database, SingletonDeps, DatabaseDeps) (*DB, error)

var _ _checkProvideDatabase = ProvideDatabase

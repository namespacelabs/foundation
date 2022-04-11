// This file was automatically generated.
package datastore

import (
	"context"

	"namespacelabs.dev/foundation/std/go/core"
	fninit "namespacelabs.dev/foundation/std/go/core/init"
	"namespacelabs.dev/foundation/std/secrets"
)

// Dependencies that are instantiated once for the lifetime of the extension
type ExtensionDeps struct {
	Cert           *secrets.Value
	Gen            *secrets.Value
	Keygen         *secrets.Value
	ReadinessCheck core.Check
}

type _checkProvideDatabase func(context.Context, fninit.Caller, *Database, *ExtensionDeps) (*DB, error)

var _ _checkProvideDatabase = ProvideDatabase

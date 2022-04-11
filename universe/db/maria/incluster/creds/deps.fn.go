// This file was automatically generated.
package creds

import (
	"context"

	fninit "namespacelabs.dev/foundation/std/go/core/init"
	"namespacelabs.dev/foundation/std/secrets"
)

// Dependencies that are instantiated once for the lifetime of the extension
type ExtensionDeps struct {
	Password *secrets.Value
}

type _checkProvideCreds func(context.Context, fninit.Caller, *CredsRequest, *ExtensionDeps) (*Creds, error)

var _ _checkProvideCreds = ProvideCreds

// This file was automatically generated.
package creds

import (
	"context"

	"namespacelabs.dev/foundation/std/secrets"
)

type ExtensionDeps struct {
	Password *secrets.Value
	User     *secrets.Value
}

type _checkProvideCreds func(context.Context, string, *CredsRequest, ExtensionDeps) (*Creds, error)

var _ _checkProvideCreds = ProvideCreds

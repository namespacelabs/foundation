// This file was automatically generated.
package creds

import (
	"context"

	"namespacelabs.dev/foundation/std/secrets"
)

// Scoped dependencies that are reinstantiated for each call to ProvideCreds
type CredsDeps struct {
	User     *secrets.Value
	Password *secrets.Value
}

type _checkProvideCreds func(context.Context, string, *CredsRequest, CredsDeps) (*Creds, error)

var _ _checkProvideCreds = ProvideCreds

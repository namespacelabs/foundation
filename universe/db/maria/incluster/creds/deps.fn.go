// This file was automatically generated.
package creds

import (
	"context"

	"namespacelabs.dev/foundation/std/secrets"
)

type SingletonDeps struct {
	Password *secrets.Value
}

type _checkProvideCreds func(context.Context, string, *CredsRequest, *SingletonDeps) (*Creds, error)

var _ _checkProvideCreds = ProvideCreds

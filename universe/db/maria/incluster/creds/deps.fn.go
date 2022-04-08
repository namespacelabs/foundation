// This file was automatically generated.
package creds

import (
	"context"

	fninit "namespacelabs.dev/foundation/std/go/core/init"
	"namespacelabs.dev/foundation/std/secrets"
)

type SingletonDeps struct {
	Password *secrets.Value
}

type _checkProvideCreds func(context.Context, fninit.Caller, *CredsRequest, *SingletonDeps) (*Creds, error)

var _ _checkProvideCreds = ProvideCreds

// This file was automatically generated.
package csrf

import (
	"context"

	"namespacelabs.dev/foundation/std/secrets"
)

type SingletonDeps struct {
	Token *secrets.Value
}

type _checkPrepare func(context.Context, *SingletonDeps) error

var _ _checkPrepare = Prepare

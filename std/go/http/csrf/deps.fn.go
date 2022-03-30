// This file was automatically generated.
package csrf

import (
	"context"

	"namespacelabs.dev/foundation/std/secrets"
)

type ExtensionDeps struct {
	Token *secrets.Value
}

type _checkPrepare func(context.Context, ExtensionDeps) error

var _ _checkPrepare = Prepare

// This file was automatically generated.
package csrf

import (
	"context"

	"namespacelabs.dev/foundation/std/secrets"
)

// Dependencies that are instantiated once for the lifetime of the extension
type ExtensionDeps struct {
	Token *secrets.Value
}

type _checkPrepare func(context.Context, ExtensionDeps) error

var _ _checkPrepare = Prepare

// This file was automatically generated.
package panicparse

import (
	"context"

	"namespacelabs.dev/foundation/std/go/core"
)

// Dependencies that are instantiated once for the lifetime of the extension.
type ExtensionDeps struct {
	DebugHandler core.DebugHandler
}

type _checkPrepare func(context.Context, ExtensionDeps) error

var _ _checkPrepare = Prepare

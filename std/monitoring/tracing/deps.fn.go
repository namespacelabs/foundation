// This file was automatically generated.
package tracing

import (
	"context"

	"namespacelabs.dev/foundation/std/go/grpc/interceptors"
)

// Dependencies that are instantiated once for the lifetime of the extension
type ExtensionDeps struct {
	Interceptors interceptors.Registration
}

type _checkPrepare func(context.Context, *ExtensionDeps) error

var _ _checkPrepare = Prepare

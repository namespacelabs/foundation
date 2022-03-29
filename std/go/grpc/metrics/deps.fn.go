// This file was automatically generated.
package metrics

import (
	"context"

	"namespacelabs.dev/foundation/std/go/grpc/interceptors"
)

type ExtensionDeps struct {
	Interceptors interceptors.Registration
}

type _checkPrepare func(context.Context, ExtensionDeps) error

var _ _checkPrepare = Prepare

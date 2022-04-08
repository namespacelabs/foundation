// This file was automatically generated.
package logging

import (
	"context"

	"namespacelabs.dev/foundation/std/go/grpc/interceptors"
)

type SingletonDeps struct {
	Interceptors interceptors.Registration
}

type _checkPrepare func(context.Context, SingletonDeps) error

var _ _checkPrepare = Prepare

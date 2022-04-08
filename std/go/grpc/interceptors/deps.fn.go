// This file was automatically generated.
package interceptors

import (
	"context"

	fninit "namespacelabs.dev/foundation/std/go/core/init"
)

type _checkProvideInterceptorRegistration func(context.Context, fninit.Caller, *InterceptorRegistration) (Registration, error)

var _ _checkProvideInterceptorRegistration = ProvideInterceptorRegistration

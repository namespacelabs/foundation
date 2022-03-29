// This file was automatically generated.
package interceptors

import (
	"context"
)

type _checkProvideInterceptorRegistration func(context.Context, string, *InterceptorRegistration) (Registration, error)

var _ _checkProvideInterceptorRegistration = ProvideInterceptorRegistration

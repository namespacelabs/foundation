// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

// This file was automatically generated.
package interceptors

import (
	"context"
)

type _checkProvideInterceptorRegistration func(context.Context, string, *InterceptorRegistration) (Registration, error)

var _ _checkProvideInterceptorRegistration = ProvideInterceptorRegistration
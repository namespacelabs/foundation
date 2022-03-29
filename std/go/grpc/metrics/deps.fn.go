// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

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
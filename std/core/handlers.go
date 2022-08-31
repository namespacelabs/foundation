// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package core

import (
	"context"

	"namespacelabs.dev/foundation/std/go/core"
)

// All `ProvideFooBarHandler` functions delegate to the implementing package.

func ProvideDebugHandler(ctx context.Context, args *DebugHandlerArgs) (core.DebugHandler, error) {
	return core.ProvideDebugHandler(ctx, args)
}

func ProvideLivenessCheck(ctx context.Context, args *LivenessCheckArgs) (core.Check, error) {
	return core.ProvideLivenessCheck(ctx, args)
}

func ProvideReadinessCheck(ctx context.Context, args *ReadinessCheckArgs) (core.Check, error) {
	return core.ProvideReadinessCheck(ctx, args)
}
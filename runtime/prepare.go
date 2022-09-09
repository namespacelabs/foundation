// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package runtime

import (
	"context"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/std/planning"
)

var prepareRegistrations = map[string]func(context.Context, planning.Context, DeferredCluster) (any, error){}

func RegisterPrepare(key string, callback func(context.Context, planning.Context, DeferredCluster) (any, error)) {
	prepareRegistrations[key] = callback
}

func Prepare(ctx context.Context, key string, env planning.Context, cluster DeferredCluster) (any, error) {
	if prepareRegistrations[key] == nil {
		return nil, fnerrors.InternalError("%s: no such runtime support", key)
	}

	return prepareRegistrations[key](ctx, env, cluster)
}

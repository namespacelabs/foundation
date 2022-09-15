// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package runtime

import (
	"context"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/std/planning"
)

var (
	prepareRegistrations      = map[string]func(context.Context, planning.Configuration, Cluster) (any, error){}
	keyedPrepareRegistrations = map[string]func(context.Context, planning.Configuration, Cluster, string) (any, error){}
)

func RegisterPrepare(key string, callback func(context.Context, planning.Configuration, Cluster) (any, error)) {
	prepareRegistrations[key] = callback
}

func RegisterKeyedPrepare(key string, callback func(context.Context, planning.Configuration, Cluster, string) (any, error)) {
	keyedPrepareRegistrations[key] = callback
}

func Prepare(ctx context.Context, key string, env planning.Configuration, cluster Cluster) (any, error) {
	if prepareRegistrations[key] == nil {
		return nil, fnerrors.InternalError("%s: no such runtime support", key)
	}

	return prepareRegistrations[key](ctx, env, cluster)
}

func PrepareKeyed(ctx context.Context, stateKey string, env planning.Configuration, cluster Cluster, key string) (any, error) {
	if prepareRegistrations[stateKey] == nil {
		return nil, fnerrors.InternalError("%s: no such runtime support", key)
	}

	return keyedPrepareRegistrations[stateKey](ctx, env, cluster, key)
}

// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package runtime

import (
	"context"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/std/cfg"
)

var (
	prepareRegistrations      = map[string]func(context.Context, cfg.Configuration, Cluster) (any, error){}
	keyedPrepareRegistrations = map[string]func(context.Context, cfg.Configuration, Cluster, string) (any, error){}
)

func RegisterPrepare(key string, callback func(context.Context, cfg.Configuration, Cluster) (any, error)) {
	prepareRegistrations[key] = callback
}

func RegisterKeyedPrepare(key string, callback func(context.Context, cfg.Configuration, Cluster, string) (any, error)) {
	keyedPrepareRegistrations[key] = callback
}

func Prepare(ctx context.Context, key string, env cfg.Configuration, cluster Cluster) (any, error) {
	if prepareRegistrations[key] == nil {
		return nil, fnerrors.InternalError("%s: no such runtime support", key)
	}

	return prepareRegistrations[key](ctx, env, cluster)
}

func PrepareKeyed(ctx context.Context, stateKey string, env cfg.Configuration, cluster Cluster, key string) (any, error) {
	if keyedPrepareRegistrations[stateKey] == nil {
		return nil, fnerrors.InternalError("%s: no runtime support", stateKey)
	}

	return keyedPrepareRegistrations[stateKey](ctx, env, cluster, key)
}

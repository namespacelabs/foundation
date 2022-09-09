// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package runtime

import (
	"context"
	"strings"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime/rtypes"
	"namespacelabs.dev/foundation/std/planning"
)

var (
	registrations = map[string]MakeRuntimeFunc{}
)

type MakeRuntimeFunc func(context.Context, planning.Context) (Class, error)

func Register(name string, r MakeRuntimeFunc) {
	registrations[strings.ToLower(name)] = r
}

func HasRuntime(name string) bool {
	_, ok := registrations[strings.ToLower(name)]
	return ok
}

// Never returns nil. If the specified runtime kind doesn't exist, then a runtime instance that always fails is returned.
func ClusterFor(ctx context.Context, env planning.Context) (Cluster, error) {
	runtime, err := obtainSpecialized[Cluster](ctx, env)
	if err != nil {
		return nil, err
	}
	return runtime, nil
}

func TargetPlatforms(ctx context.Context, env planning.Context) ([]specs.Platform, error) {
	runtime, err := obtainSpecialized[HasTargetPlatforms](ctx, env)
	if err != nil {
		return nil, err
	}
	return runtime.TargetPlatforms(ctx)
}

func PrepareProvision(ctx context.Context, env planning.Context) (*rtypes.ProvisionProps, error) {
	runtime, err := obtainSpecialized[HasPrepareProvision](ctx, env)
	if err != nil {
		return nil, err
	}
	return runtime.PrepareProvision(ctx, env)
}

func DeferredFor(ctx context.Context, env planning.Context) (Class, error) {
	rt := strings.ToLower(env.Environment().Runtime)
	if obtain, ok := registrations[rt]; ok {
		r, err := obtain(ctx, env)
		if err != nil {
			return nil, err
		}

		return r, nil
	}

	return nil, fnerrors.InternalError("%s: no such runtime", rt)
}

func obtainSpecialized[V any](ctx context.Context, env planning.Context) (V, error) {
	var empty V

	deferred, err := DeferredFor(ctx, env)
	if err != nil {
		return empty, err
	}

	if h, ok := deferred.(V); ok {
		return h, nil
	}

	cluster, err := deferred.AttachToCluster(ctx, deferred.Namespace(env))
	if err != nil {
		return empty, err
	}

	return cluster.(V), nil
}

// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package runtime

import (
	"context"
	"strings"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/std/planning"
)

var (
	registrations = map[string]InstantiateClassFunc{}
)

type InstantiateClassFunc func(context.Context, planning.Configuration) (Class, error)

func Register(name string, r InstantiateClassFunc) {
	registrations[strings.ToLower(name)] = r
}

func HasRuntime(name string) bool {
	_, ok := registrations[strings.ToLower(name)]
	return ok
}

func ClusterFor(ctx context.Context, env planning.Context) (Cluster, error) {
	deferred, err := ClassFor(ctx, env)
	if err != nil {
		return nil, err
	}

	return deferred.AttachToCluster(ctx, env.Configuration())
}

func PlannerFor(ctx context.Context, env planning.Context) (Planner, error) {
	cluster, err := ClusterFor(ctx, env)
	if err != nil {
		return nil, err
	}

	return cluster.Planner(env), nil
}

func NamespaceFor(ctx context.Context, env planning.Context) (ClusterNamespace, error) {
	cluster, err := ClusterFor(ctx, env)
	if err != nil {
		return nil, err
	}

	return cluster.Bind(env)
}

func ClassFor(ctx context.Context, env planning.Context) (Class, error) {
	rt := strings.ToLower(env.Environment().Runtime)
	if obtain, ok := registrations[rt]; ok {
		r, err := obtain(ctx, env.Configuration())
		if err != nil {
			return nil, err
		}

		return r, nil
	}

	return nil, fnerrors.InternalError("%s: no such runtime", rt)
}

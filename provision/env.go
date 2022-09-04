// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package provision

import (
	"context"

	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace"
)

type boundEnv struct {
	planning.Context
	workspace.SealedPackages
}

var _ ServerEnv = boundEnv{}

func BindPlanWithPackages(env planning.Context, pr workspace.SealedPackages) boundEnv {
	return boundEnv{Context: env, SealedPackages: pr}
}

func RequireServer(ctx context.Context, env planning.Context, pkgname schema.PackageName) (Server, error) {
	return RequireServerWith(ctx, env, workspace.NewPackageLoader(env), pkgname)
}

func RequireServerWith(ctx context.Context, env planning.Context, pl *workspace.PackageLoader, pkgname schema.PackageName) (Server, error) {
	return makeServer(ctx, pl, env.Environment(), pkgname, func() ServerEnv {
		return BindPlanWithPackages(env, pl.Seal())
	})
}

func RequireLoadedServer(ctx context.Context, e ServerEnv, pkgname schema.PackageName) (Server, error) {
	return makeServer(ctx, e, e.Environment(), pkgname, func() ServerEnv {
		return e
	})
}

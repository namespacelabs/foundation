// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package provision

import (
	"context"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
)

type Env struct {
	root *workspace.Root
	env  *schema.Environment
}

type boundEnv struct {
	root *workspace.Root
	env  *schema.Environment
	sp   workspace.SealedPackages
}

var _ ServerEnv = boundEnv{}

func (e Env) ErrorLocation() string               { return e.root.Workspace.ModuleName }
func (e Env) Root() *workspace.Root               { return e.root }
func (e Env) Workspace() *schema.Workspace        { return e.root.Workspace }
func (e Env) DevHost() *schema.DevHost            { return e.root.DevHost }
func (e Env) Proto() *schema.Environment          { return e.env }
func (e Env) Name() string                        { return e.env.Name }
func (e Env) Runtime() string                     { return e.env.Runtime }
func (e Env) Purpose() schema.Environment_Purpose { return e.env.Purpose }

func (e Env) RequireServerAtLoc(ctx context.Context, loc fnfs.Location) (Server, error) {
	return e.RequireServer(ctx, loc.AsPackageName())
}

func (e Env) RequireServer(ctx context.Context, pkgname schema.PackageName) (Server, error) {
	return e.RequireServerWith(ctx, workspace.NewPackageLoader(e.root), pkgname)
}

func (e Env) RequireServerWith(ctx context.Context, pl *workspace.PackageLoader, pkgname schema.PackageName) (Server, error) {
	return makeServer(ctx, pl, e.Proto(), pkgname, func() ServerEnv {
		return e.BindWith(pl.Seal())
	})
}

func (e Env) BindWith(pr workspace.SealedPackages) boundEnv {
	return boundEnv{e.root, e.env, pr}
}

func RequireServer(ctx context.Context, e ServerEnv, pkgname schema.PackageName) (Server, error) {
	return makeServer(ctx, e, e.Proto(), pkgname, func() ServerEnv {
		return e
	})
}

func (e boundEnv) ErrorLocation() string        { return e.root.Workspace.ModuleName }
func (e boundEnv) Workspace() *schema.Workspace { return e.root.Workspace }
func (e boundEnv) DevHost() *schema.DevHost     { return e.root.DevHost }
func (e boundEnv) Proto() *schema.Environment   { return e.env }

func (e boundEnv) Resolve(ctx context.Context, packageName schema.PackageName) (workspace.Location, error) {
	return e.sp.Resolve(ctx, packageName)
}
func (e boundEnv) LoadByName(ctx context.Context, packageName schema.PackageName) (*workspace.Package, error) {
	return e.sp.LoadByName(ctx, packageName)
}
func (e boundEnv) Sources() []workspace.ModuleSources {
	return e.sp.Sources()
}

func RequireEnv(root *workspace.Root, name string) (Env, error) {
	available := root.Workspace.Env

	if available == nil {
		available = []*schema.Environment{
			{
				Name:    "dev",
				Runtime: "kubernetes", // XXX
				Purpose: schema.Environment_DEVELOPMENT,
			},
			{
				Name:    "prod",
				Runtime: "kubernetes",
				Purpose: schema.Environment_PRODUCTION,
			},
		}
	}

	for _, env := range available {
		if env.Name == name {
			return MakeEnv(root, env), nil
		}
	}

	return Env{}, fnerrors.UserError(nil, "no such environment: %s", name)
}

func MakeEnv(root *workspace.Root, env *schema.Environment) Env {
	return Env{root: root, env: env}
}

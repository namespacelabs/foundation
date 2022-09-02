// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package provision

import (
	"context"

	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
)

type Env struct {
	errorLocation string
	workspace     *schema.Workspace
	loadedFrom    *schema.Workspace_LoadedFrom
	devHost       *schema.DevHost
	env           *schema.Environment
}

type boundEnv struct {
	parent Env
	env    *schema.Environment
	sp     workspace.SealedPackages
}

var _ ServerEnv = boundEnv{}

func (e Env) ErrorLocation() string                             { return e.errorLocation }
func (e Env) WorkspaceAbsPath() string                          { return e.loadedFrom.AbsPath }
func (e Env) Workspace() *schema.Workspace                      { return e.workspace }
func (e Env) WorkspaceLoadedFrom() *schema.Workspace_LoadedFrom { return e.loadedFrom }
func (e Env) DevHost() *schema.DevHost                          { return e.devHost }
func (e Env) Proto() *schema.Environment                        { return e.env }
func (e Env) Name() string                                      { return e.env.Name }
func (e Env) Runtime() string                                   { return e.env.Runtime }
func (e Env) Purpose() schema.Environment_Purpose               { return e.env.Purpose }

func (e Env) RequireServerAtLoc(ctx context.Context, loc fnfs.Location) (Server, error) {
	return e.RequireServer(ctx, loc.AsPackageName())
}

func (e Env) RequireServer(ctx context.Context, pkgname schema.PackageName) (Server, error) {
	return e.RequireServerWith(ctx, workspace.NewPackageLoader(e), pkgname)
}

func (e Env) RequireServerWith(ctx context.Context, pl *workspace.PackageLoader, pkgname schema.PackageName) (Server, error) {
	return makeServer(ctx, pl, e.Proto(), pkgname, func() ServerEnv {
		return e.BindWith(pl.Seal())
	})
}

func (e Env) BindWith(pr workspace.SealedPackages) boundEnv {
	return boundEnv{e, e.env, pr}
}

func RequireServer(ctx context.Context, e ServerEnv, pkgname schema.PackageName) (Server, error) {
	return makeServer(ctx, e, e.Proto(), pkgname, func() ServerEnv {
		return e
	})
}

func (e boundEnv) ErrorLocation() string        { return e.parent.ErrorLocation() }
func (e boundEnv) Workspace() *schema.Workspace { return e.parent.Workspace() }
func (e boundEnv) WorkspaceLoadedFrom() *schema.Workspace_LoadedFrom {
	return e.parent.WorkspaceLoadedFrom()
}
func (e boundEnv) DevHost() *schema.DevHost   { return e.parent.DevHost() }
func (e boundEnv) Proto() *schema.Environment { return e.env }

func (e boundEnv) Resolve(ctx context.Context, packageName schema.PackageName) (workspace.Location, error) {
	return e.sp.Resolve(ctx, packageName)
}
func (e boundEnv) LoadByName(ctx context.Context, packageName schema.PackageName) (*workspace.Package, error) {
	return e.sp.LoadByName(ctx, packageName)
}
func (e boundEnv) Ensure(ctx context.Context, packageName schema.PackageName) error {
	return e.sp.Ensure(ctx, packageName)
}
func (e boundEnv) Sources() []workspace.ModuleSources {
	return e.sp.Sources()
}

func EnvsOrDefault(devHost *schema.DevHost, workspace *schema.Workspace) []*schema.Environment {
	baseEnvs := slices.Clone(devHost.LocalEnv)

	if workspace.Env != nil {
		return append(baseEnvs, workspace.Env...)
	}

	return append(baseEnvs, []*schema.Environment{
		{
			Name:    "dev",
			Runtime: "kubernetes", // XXX
			Purpose: schema.Environment_DEVELOPMENT,
		},
		{
			Name:    "staging",
			Runtime: "kubernetes",
			Purpose: schema.Environment_PRODUCTION,
		},
		{
			Name:    "prod",
			Runtime: "kubernetes",
			Purpose: schema.Environment_PRODUCTION,
		},
	}...)
}

func RequireEnv(root *workspace.Root, name string) (Env, error) {
	for _, env := range EnvsOrDefault(root.DevHost(), root.Workspace()) {
		if env.Name == name {
			return MakeEnv(root, env), nil
		}
	}

	return Env{}, fnerrors.UserError(nil, "no such environment: %s", name)
}

func RequireEnvWith(parent ops.Environment, name string) (Env, error) {
	for _, env := range EnvsOrDefault(parent.DevHost(), parent.Workspace()) {
		if env.Name == name {
			return MakeEnvWith(parent.Workspace(), parent.WorkspaceLoadedFrom(), parent.DevHost(), env), nil
		}
	}

	return Env{}, fnerrors.UserError(nil, "no such environment: %s", name)
}

func MakeEnv(root *workspace.Root, env *schema.Environment) Env {
	return Env{errorLocation: root.ErrorLocation(), workspace: root.Workspace(), loadedFrom: root.WorkspaceLoadedFrom(), devHost: root.DevHost(), env: env}
}

func MakeEnvWith(ws *schema.Workspace, lf *schema.Workspace_LoadedFrom, devhost *schema.DevHost, env *schema.Environment) Env {
	return Env{errorLocation: ws.ModuleName, workspace: ws, loadedFrom: lf, devHost: devhost, env: env}
}

func MakeEnvFromEnv(env ops.Environment) Env {
	return Env{errorLocation: env.ErrorLocation(), workspace: env.Workspace(), loadedFrom: env.WorkspaceLoadedFrom(), devHost: env.DevHost(), env: env.Proto()}
}

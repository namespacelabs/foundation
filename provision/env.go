// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package provision

import (
	"context"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/planning"
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
	planning.Context
	packages workspace.SealedPackages
}

var _ ServerEnv = boundEnv{}

func (e Env) ErrorLocation() string                             { return e.errorLocation }
func (e Env) Workspace() *schema.Workspace                      { return e.workspace }
func (e Env) WorkspaceLoadedFrom() *schema.Workspace_LoadedFrom { return e.loadedFrom }
func (e Env) DevHost() *schema.DevHost                          { return e.devHost }
func (e Env) Environment() *schema.Environment                  { return e.env }

func BindPlanWithPackages(env planning.Context, pr workspace.SealedPackages) boundEnv {
	return boundEnv{Context: env, packages: pr}
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

func (e boundEnv) Resolve(ctx context.Context, packageName schema.PackageName) (workspace.Location, error) {
	return e.packages.Resolve(ctx, packageName)
}
func (e boundEnv) LoadByName(ctx context.Context, packageName schema.PackageName) (*workspace.Package, error) {
	return e.packages.LoadByName(ctx, packageName)
}
func (e boundEnv) Ensure(ctx context.Context, packageName schema.PackageName) error {
	return e.packages.Ensure(ctx, packageName)
}
func (e boundEnv) Sources() []workspace.ModuleSources {
	return e.packages.Sources()
}

func RequireEnv(root *workspace.Root, name string) (Env, error) {
	for _, env := range planning.EnvsOrDefault(root.DevHost(), root.Workspace()) {
		if env.Name == name {
			return MakeEnv(root, env), nil
		}
	}

	return Env{}, fnerrors.UserError(nil, "no such environment: %s", name)
}

func RequireEnvWith(parent planning.Context, name string) (Env, error) {
	for _, env := range planning.EnvsOrDefault(parent.DevHost(), parent.Workspace()) {
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

func MakeEnvFromEnv(env planning.Context) Env {
	return Env{errorLocation: env.ErrorLocation(), workspace: env.Workspace(), loadedFrom: env.WorkspaceLoadedFrom(), devHost: env.DevHost(), env: env.Environment()}
}

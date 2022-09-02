// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package codegen

import (
	"context"

	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/languages"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
)

// ForNodeLocations generates protos for Extensions and Services. Locations in `locs` are sorted in a topological order.
func ForLocationsGenProto(ctx context.Context, root *workspace.Root, locs []fnfs.Location, onError func(fnerrors.CodegenError)) error {
	pl := workspace.NewPackageLoader(root, nil /* env */)
	g := ops.Plan{}
	for _, loc := range locs {
		pkg, err := pl.LoadByNameWithOpts(ctx, loc.AsPackageName(), workspace.DontLoadDependencies())
		if err != nil {
			onError(fnerrors.CodegenError{PackageName: loc.AsPackageName().String(), What: "loading schema", Err: err})
			continue
		}
		if n := pkg.Node(); n != nil {
			defs, err := ProtosForNode(pkg)
			if err != nil {
				onError(fnerrors.CodegenError{PackageName: loc.AsPackageName().String(), What: "generate node", Err: err})
			} else {
				if err := g.Add(defs...); err != nil {
					return err
				}
			}
		}
		if _, err := g.Execute(ctx, "workspace.generate.phase.node", genEnv{root, pl.Seal()}); err != nil {
			return err
		}
	}
	return nil
}

// ForLocationsGenCode generates code for all packages in `locs`. At this stage we assume protos are already generated.
func ForLocationsGenCode(ctx context.Context, root *workspace.Root, locs []fnfs.Location, onError func(fnerrors.CodegenError)) error {
	pl := workspace.NewPackageLoader(root, nil /* env */)
	g := ops.Plan{}
	for _, loc := range locs {
		sealed, err := workspace.Seal(ctx, pl, loc.AsPackageName(), nil)
		if err != nil {
			onError(fnerrors.CodegenError{PackageName: loc.AsPackageName().String(), What: "loading schema", Err: err})
			continue
		}
		if srv := sealed.Proto.Server; srv != nil {
			if srv.Integration != nil {
				// Opaque.
				continue
			}

			defs, err := languages.IntegrationFor(srv.Framework).GenerateServer(sealed.ParsedPackage, sealed.Proto.Node)
			if err != nil {
				onError(fnerrors.CodegenError{PackageName: loc.AsPackageName().String(), What: "generate server", Err: err})
			} else {
				if err := g.Add(defs...); err != nil {
					return err
				}
			}
		} else {
			var pkg *workspace.Package
			for _, dep := range sealed.Deps {
				if dep.PackageName() == loc.AsPackageName() {
					pkg = dep
					break
				}
			}

			if pkg == nil || pkg.Node() == nil {
				continue
			}

			defs, err := ForNodeForLanguage(pkg, sealed.Proto.Node)
			if err != nil {
				onError(fnerrors.CodegenError{PackageName: loc.AsPackageName().String(), What: "generate node", Err: err})
				return err
			} else {
				if err := g.Add(defs...); err != nil {
					return err
				}
			}
		}
	}
	_, err := g.Execute(ctx, "workspace.generate.phase.code", genEnv{root, pl.Seal()})
	return err
}

type genEnv struct {
	root *workspace.Root
	r    workspace.Packages
}

var _ workspace.WorkspaceEnvironment = genEnv{}

func (g genEnv) ErrorLocation() string        { return g.root.ErrorLocation() }
func (g genEnv) OutputFS() fnfs.ReadWriteFS   { return g.root.FS() }
func (g genEnv) ModuleName() string           { return g.root.Workspace().ModuleName }
func (g genEnv) Proto() *schema.Environment   { return nil }
func (g genEnv) Root() *workspace.Root        { return g.root }
func (g genEnv) Workspace() *schema.Workspace { return g.root.Workspace() }
func (g genEnv) DevHost() *schema.DevHost     { return g.root.DevHost() }
func (g genEnv) WorkspaceLoadedFrom() *schema.Workspace_LoadedFrom {
	return g.root.WorkspaceLoadedFrom()
}

func (g genEnv) Resolve(ctx context.Context, pkg schema.PackageName) (workspace.Location, error) {
	return g.r.Resolve(ctx, pkg)
}

func (g genEnv) LoadByName(ctx context.Context, packageName schema.PackageName) (*workspace.Package, error) {
	return g.r.LoadByName(ctx, packageName)
}

func (g genEnv) Ensure(ctx context.Context, packageName schema.PackageName) error {
	return g.r.Ensure(ctx, packageName)
}

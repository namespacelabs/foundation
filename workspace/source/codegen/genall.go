// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package codegen

import (
	"context"

	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/languages"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
)

type GenerateError struct {
	PackageName schema.PackageName
	What        string
	Err         error
}

func ForLocations(ctx context.Context, root *workspace.Root, locs []fnfs.Location, onError func(GenerateError)) error {
	var errCount int
	var g ops.Runner

	pl := workspace.NewPackageLoader(root)

	for _, loc := range locs {
		sealed, err := workspace.Seal(ctx, pl, loc.AsPackageName(), nil)
		if err != nil {
			onError(GenerateError{PackageName: loc.AsPackageName(), What: "loading schema", Err: err})
			errCount++
		} else {
			if srv := sealed.Proto.Server; srv != nil {
				defs, err := languages.IntegrationFor(srv.Framework).GenerateServer(sealed.ParsedPackage, sealed.Proto.Node)
				if err != nil {
					onError(GenerateError{PackageName: loc.AsPackageName(), What: "generate server", Err: err})
					errCount++
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

				defs, err := ForNode(pkg, sealed.Proto.Node)
				if err != nil {
					onError(GenerateError{PackageName: loc.AsPackageName(), What: "generate node", Err: err})
					errCount++
				} else {
					if err := g.Add(defs...); err != nil {
						return err
					}
				}
			}
		}
	}

	_, err := g.Apply(ctx, "workspace.generate", genEnv{root, pl.Seal()})
	return err
}

type genEnv struct {
	root *workspace.Root
	r    workspace.Packages
}

var _ workspace.WorkspaceEnvironment = genEnv{}

func (g genEnv) ErrorLocation() string        { return g.root.ErrorLocation() }
func (g genEnv) OutputFS() fnfs.ReadWriteFS   { return g.root.FS() }
func (g genEnv) Proto() *schema.Environment   { return nil }
func (g genEnv) Root() *workspace.Root        { return g.root }
func (g genEnv) Workspace() *schema.Workspace { return g.root.Workspace }
func (g genEnv) DevHost() *schema.DevHost     { return g.root.DevHost }

func (g genEnv) Resolve(ctx context.Context, pkg schema.PackageName) (workspace.Location, error) {
	return g.r.Resolve(ctx, pkg)
}

func (g genEnv) LoadByName(ctx context.Context, packageName schema.PackageName) (*workspace.Package, error) {
	return g.r.LoadByName(ctx, packageName)
}

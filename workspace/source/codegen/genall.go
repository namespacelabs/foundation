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
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace"
)

// ForNodeLocations generates protos for Extensions and Services. Locations in `locs` are sorted in a topological order.
func ForLocationsGenProto(ctx context.Context, out pkggraph.MutableModule, env planning.Context, locs []fnfs.Location, onError func(fnerrors.CodegenError)) error {
	pl := workspace.NewPackageLoader(env)
	g := ops.NewEmptyPlan()
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
				g.Add(defs...)
			}
		}
		if err := ops.Execute(ctx, env, "workspace.generate.phase.node", g, nil,
			pkggraph.MutableModuleInjection.With(out),
			pkggraph.PackageLoaderInjection.With(pl.Seal()),
		); err != nil {
			return err
		}
	}
	return nil
}

// ForLocationsGenCode generates code for all packages in `locs`. At this stage we assume protos are already generated.
func ForLocationsGenCode(ctx context.Context, out pkggraph.MutableModule, env planning.Context, locs []fnfs.Location, onError func(fnerrors.CodegenError)) error {
	pl := workspace.NewPackageLoader(env)
	g := ops.NewEmptyPlan()
	for _, loc := range locs {
		sealed, err := workspace.Seal(ctx, pl, loc.AsPackageName(), nil)
		if err != nil {
			onError(fnerrors.CodegenError{PackageName: loc.AsPackageName().String(), What: "loading schema", Err: err})
			continue
		}
		if srv := sealed.Proto.Server; srv != nil {
			defs, err := languages.IntegrationFor(srv.Framework).GenerateServer(sealed.ParsedPackage, sealed.Proto.Node)
			if err != nil {
				onError(fnerrors.CodegenError{PackageName: loc.AsPackageName().String(), What: "generate server", Err: err})
			} else {
				g.Add(defs...)
			}
		} else {
			var pkg *pkggraph.Package
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
				g.Add(defs...)
			}
		}
	}

	return ops.Execute(ctx, env, "workspace.generate.phase.code", g, nil,
		pkggraph.MutableModuleInjection.With(out),
		pkggraph.PackageLoaderInjection.With(pl.Seal()),
	)
}

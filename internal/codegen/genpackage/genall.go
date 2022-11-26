// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package genpackage

import (
	"context"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/integrations"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/execution"
	"namespacelabs.dev/foundation/std/pkggraph"
)

// ForNodeLocations generates protos for Extensions and Services. Locations in `locs` are sorted in a topological order.
func ForLocationsGenProto(ctx context.Context, out pkggraph.MutableModule, env cfg.Context, locs []fnfs.Location, onError func(fnerrors.CodegenError)) error {
	pl := parsing.NewPackageLoader(env)
	g := execution.NewEmptyPlan()
	for _, loc := range locs {
		pkg, err := pl.LoadByName(ctx, loc.AsPackageName())
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
		if err := execution.Execute(ctx, "workspace.generate.phase.node", g, nil,
			execution.FromContext(env),
			pkggraph.MutableModuleInjection.With(out),
			pkggraph.PackageLoaderInjection.With(pl.Seal()),
		); err != nil {
			return err
		}
	}
	return nil
}

// ForLocationsGenCode generates code for all packages in `locs`. At this stage we assume protos are already generated.
func ForLocationsGenCode(ctx context.Context, out pkggraph.MutableModule, env cfg.Context, locs []fnfs.Location, onError func(fnerrors.CodegenError)) error {
	pl := parsing.NewPackageLoader(env)

	g := execution.NewEmptyPlan()
	for _, loc := range locs {
		sealed, err := parsing.Seal(ctx, pl, loc.AsPackageName(), nil)
		if err != nil {
			onError(fnerrors.CodegenError{PackageName: loc.AsPackageName().String(), What: "loading schema", Err: err})
			continue
		}

		if srv := sealed.Proto.Server; srv != nil {
			defs, err := integrations.IntegrationFor(srv.Framework).GenerateServer(sealed.ParsedPackage, sealed.Proto.Node)
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

	return execution.Execute(ctx, "workspace.generate.phase.code", g, nil,
		execution.FromContext(env),
		pkggraph.MutableModuleInjection.With(out),
		pkggraph.PackageLoaderInjection.With(pl.Seal()),
	)
}

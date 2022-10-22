// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cmd

import (
	"context"

	"github.com/philopon/go-toposort"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/codegen/genpackage"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/internal/parsing/module"
	"namespacelabs.dev/foundation/internal/uniquestrings"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
)

func NewGenerateCmd() *cobra.Command {
	var (
		env cfg.Context
	)

	return fncobra.
		Cmd(&cobra.Command{
			Use:     "generate",
			Short:   "Generate service and server glue code, for each of the known schemas.",
			Long:    "Generate service and server glue code, for each of the known schemas.\nAutomatically invoked with `build` and `deploy`.",
			Aliases: []string{"gen"},
			Args:    cobra.NoArgs,
			Hidden:  true,
		}).
		With(fncobra.HardcodeEnv(&env, "dev")).
		Do(func(ctx context.Context) error {
			root, err := module.FindRoot(ctx, ".")
			if err != nil {
				return err
			}

			errorCollector := fnerrors.ErrorCollector{}

			if err := generateProtos(ctx, env, root, errorCollector.Append); err != nil {
				return err
			}

			list, err := parsing.ListSchemas(ctx, env, root)
			if err != nil {
				return err
			}

			// Generate code.
			if err := genpackage.ForLocationsGenCode(ctx, root, env, list.Locations, errorCollector.Append); err != nil {
				return err
			}

			return errorCollector.Error()
		})
}

func generateProtos(ctx context.Context, env cfg.Context, root *parsing.Root, handleGenErr func(fnerrors.CodegenError)) error {
	list, err := parsing.ListSchemas(ctx, env, root)
	if err != nil {
		return err
	}

	pl := parsing.NewPackageLoader(env)
	wl := cuefrontend.WorkspaceLoader{PackageLoader: pl}

	cuePackages := map[string]*fncue.CuePackage{} // Cue packages by PackageName.

	var nodeLocs []fnfs.Location
	for k, loc := range list.Locations {
		if !(list.Types[k] == parsing.PackageType_Extension || list.Types[k] == parsing.PackageType_Service) {
			continue
		}

		if err := fncue.CollectImports(ctx, wl, loc.AsPackageName().String(), cuePackages); err != nil {
			return err
		}

		nodeLocs = append(nodeLocs, loc)
	}

	imports := map[schema.PackageName]uniquestrings.List{}
	for packageName, cp := range cuePackages {
		l := &uniquestrings.List{}
		for _, imp := range cp.Imports {
			l.Add(imp)
		}

		imports[schema.PackageName(packageName)] = *l
	}

	topoSorted, err := topoSortNodes(nodeLocs, imports)
	if err != nil {
		return err
	}

	return genpackage.ForLocationsGenProto(ctx, root, env, topoSorted, handleGenErr)
}

func topoSortNodes(nodes []fnfs.Location, imports map[schema.PackageName]uniquestrings.List) ([]fnfs.Location, error) {
	// Gather all the possible nodes into a set.
	all := &uniquestrings.List{}
	for _, from := range nodes {
		parent := from.AsPackageName()
		all.Add(parent.String())
		if imp, ok := imports[parent]; ok {
			for _, i := range imp.Strings() {
				all.Add(i)
			}
		}
	}
	graph := toposort.NewGraph(all.Len())

	for _, n := range all.Strings() {
		graph.AddNode(n)
	}
	for _, from := range nodes {
		parent := from.AsPackageName()
		if children, ok := imports[parent]; ok {
			for _, child := range children.Strings() {
				// First children then parents.
				graph.AddEdge(child, parent.String())
			}
		}
	}

	result, solved := graph.Toposort()
	if !solved {
		return nil, fnerrors.InternalError("dependency graph has a cycle")
	}

	// Maps package name to the original place in list.Locations.
	packageToLoc := map[schema.PackageName]fnfs.Location{}
	for _, loc := range nodes {
		packageToLoc[loc.AsPackageName()] = loc
	}
	end := make([]fnfs.Location, 0, len(nodes))
	for _, k := range result {
		if loc, ok := packageToLoc[schema.PackageName(k)]; ok {
			end = append(end, loc)
		}
	}

	return end, nil
}

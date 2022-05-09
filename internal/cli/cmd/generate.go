// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cmd

import (
	"context"

	"github.com/philopon/go-toposort"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/internal/uniquestrings"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/module"
	"namespacelabs.dev/foundation/workspace/source/codegen"
)

func NewGenerateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "generate",
		Short:   "Generate service and server glue code, for each of the known schemas.",
		Long:    "Generate service and server glue code, for each of the known schemas.\nAutomatically invoked with `build` and `deploy`.",
		Aliases: []string{"gen"},
		Args:    cobra.NoArgs,

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			root, err := module.FindRoot(ctx, ".")
			if err != nil {
				return err
			}
			if err := generateProtos(ctx, root); err != nil {
				return err
			}

			list, err := workspace.ListSchemas(ctx, root)
			if err != nil {
				return err
			}
			// Generate code.
			return codegen.ForLocationsGenCode(ctx, root, list.Locations, func(e codegen.GenerateError) {
				w := console.Stderr(ctx)
				fnerrors.Format(w, true, e.Err)
			})
		}),
	}

	return cmd
}

func generateProtos(ctx context.Context, root *workspace.Root) error {
	list, err := workspace.ListSchemasWithOpt(ctx, root, workspace.ListSchemaOpt{NodeOnly: true})
	if err != nil {
		return err
	}

	pl := workspace.NewPackageLoader(root)
	wl := cuefrontend.WorkspaceLoader{Pl: pl}

	cuePackages := make(map[string]*fncue.CuePackage) // Cue packages by PackageName.
	for _, loc := range list.Locations {
		if err := fncue.CollectImports(ctx, wl, loc.AsPackageName().String(), cuePackages); err != nil {
			return err
		}
	}
	imports := map[schema.PackageName]uniquestrings.List{}
	for packageName, cp := range cuePackages {
		l := &uniquestrings.List{}
		for _, imp := range cp.Imports {
			l.Add(imp)
		}
		imports[schema.PackageName(packageName)] = *l
	}
	// Maps package name to the original place in list.Locations.
	packageIndex := map[schema.PackageName]uint64{}
	for i, loc := range list.Locations {
		packageIndex[loc.AsPackageName()] = uint64(i)
	}
	topoSorted, err := topoSortNodes(list.Locations, imports, packageIndex)
	if err != nil {
		return err
	}
	return codegen.ForLocationsGenProto(ctx, root, topoSorted, func(e codegen.GenerateError) {
		w := console.Stderr(ctx)
		fnerrors.Format(w, true, e.Err)
	})
}

func topoSortNodes(nodes []fnfs.Location, imports map[schema.PackageName]uniquestrings.List, pkgIdx map[schema.PackageName]uint64) ([]fnfs.Location, error) {
	// Gather all the possible nodes into a set.
	all := map[string]struct{}{}
	for _, from := range nodes {
		parent := from.AsPackageName()
		all[parent.String()] = struct{}{}
		if imp, ok := imports[parent]; ok {
			for _, i := range imp.Strings() {
				all[i] = struct{}{}
			}
		}
	}
	graph := toposort.NewGraph(len(all))

	for n := range all {
		graph.AddNode(n)
	}
	for _, from := range nodes {
		parent := from.AsPackageName()
		if children, ok := imports[parent]; ok {
			for _, child := range children.Strings() {
				graph.AddEdge(child, parent.String())
			}
		}
	}

	result, solved := graph.Toposort()
	if !solved {
		return nil, fnerrors.InternalError("ops dependencies are not solvable")
	}

	end := make([]fnfs.Location, 0, len(nodes))
	for _, k := range result {
		if i, ok := pkgIdx[schema.PackageName(k)]; ok {
			end = append(end, nodes[i])
		}
	}

	return end, nil
}

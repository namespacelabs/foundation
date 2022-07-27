// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package fncobra

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/module"
)

type Locations struct {
	Locs []fnfs.Location
	Root *workspace.Root
	// Whether the user explicitly specified a list of locations.
	// If true, "All" can be not empty if "DefaultToAllWhenEmpty" is true
	AreSpecified bool
}

type LocationsParser struct {
	locsOut         *Locations
	opts            *ParseLocationsOpts
	usePackageNames bool
}

type ParseLocationsOpts struct {
	// Verify that exactly one location is specified.
	RequireSingle bool
	// If true, and no locations are specified, then "workspace.ListSchemas" result is used.
	DefaultToAllWhenEmpty bool
}

func ParseLocations(locsOut *Locations, opts *ParseLocationsOpts) *LocationsParser {
	return &LocationsParser{
		locsOut: locsOut,
		opts:    opts,
	}
}

func (p *LocationsParser) AddFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&p.usePackageNames, "use_package_names", p.usePackageNames, "Specify locations by using their fully qualified package name instead.")
}

func (p *LocationsParser) Parse(ctx context.Context, args []string) error {
	if p.locsOut == nil {
		return fnerrors.InternalError("locsOut must be set")
	}

	root, err := module.FindRoot(ctx, ".")
	if err != nil {
		return err
	}

	locs, err := packagesFromArgs(ctx, root, p.usePackageNames, args)
	if err != nil {
		return err
	}

	if p.opts.RequireSingle && len(locs) != 1 {
		return fnerrors.UserError(nil, "expected exactly one package")
	}

	if p.opts.DefaultToAllWhenEmpty && len(locs) == 0 {
		schemaList, err := workspace.ListSchemas(ctx, root)
		if err != nil {
			return err
		}

		locs = schemaList.Locations
	}

	*p.locsOut = Locations{
		Root:         root,
		Locs:         locs,
		AreSpecified: len(args) > 0,
	}

	return nil
}

func packagesFromArgs(ctx context.Context, root *workspace.Root, usePackageNames bool, args []string) ([]fnfs.Location, error) {
	var locations []fnfs.Location
	pl := workspace.NewPackageLoader(root)
	for _, arg := range args {
		var fsloc fnfs.Location
		if usePackageNames {
			loc, err := pl.Resolve(ctx, schema.PackageName(arg))
			if err != nil {
				return nil, err
			}
			fsloc = fnfs.Location{
				ModuleName: loc.Module.ModuleName(),
				RelPath:    loc.Rel(),
			}
		} else {
			// XXX RelPackage should probably validate that it's a valid path (e.g. doesn't escape module).
			fsloc = root.RelPackage(arg)
		}

		locations = append(locations, fsloc)
	}

	return locations, nil
}

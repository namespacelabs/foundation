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
	locs []fnfs.Location
}

func (l *Locations) All() []fnfs.Location { return l.locs }
func (l *Locations) AssertSingle() (fnfs.Location, error) {
	if len(l.locs) != 1 {
		return fnfs.Location{}, fnerrors.UserError(nil, "expected exactly one package")
	}
	return l.locs[0], nil
}

type LocationsParser struct {
	locsOut         *Locations
	usePackageNames bool
}

func NewLocationsParser(locsOut *Locations) *LocationsParser {
	return &LocationsParser{
		locsOut: locsOut,
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

	*p.locsOut = Locations{locs: locs}

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

// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package fncobra

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/std/planning"
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
	locsOut *Locations
	opts    *ParseLocationsOpts
	env     *planning.Context
}

type ParseLocationsOpts struct {
	// Verify that exactly one location is specified.
	RequireSingle bool
	// If true, and no locations are specified, then "workspace.ListSchemas" result is used.
	DefaultToAllWhenEmpty bool
}

func ParseLocations(locsOut *Locations, env *planning.Context, opts *ParseLocationsOpts) *LocationsParser {
	return &LocationsParser{
		locsOut: locsOut,
		opts:    opts,
		env:     env,
	}
}

func (p *LocationsParser) AddFlags(cmd *cobra.Command) {}

func (p *LocationsParser) Parse(ctx context.Context, args []string) error {
	if p.locsOut == nil {
		return fnerrors.InternalError("locsOut must be set")
	}

	root, err := module.FindRoot(ctx, ".")
	if err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	relCwd, err := filepath.Rel(root.Abs(), cwd)
	if err != nil {
		return err
	}

	locs, err := locationsFromArgs(root.Workspace().ModuleName(), root.Workspace().Proto().AllReferencedModules(), relCwd, args)
	if err != nil {
		return err
	}

	if p.opts.RequireSingle && len(locs) != 1 {
		return fnerrors.UserError(nil, "expected exactly one package")
	}

	if p.opts.DefaultToAllWhenEmpty && len(locs) == 0 {
		schemaList, err := workspace.ListSchemas(ctx, *p.env, root)
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

func locationsFromArgs(mainModuleName string, moduleNames []string, relCwd string, args []string) ([]fnfs.Location, error) {
	var locations []fnfs.Location
	for _, arg := range args {
		if filepath.IsAbs(arg) {
			return nil, fnerrors.UserError(nil, "absolute paths are not supported: %s", arg)
		}

		var rel string
		var moduleName string
		for _, m := range moduleNames {
			modulePrefix := m + "/"
			if strings.HasPrefix(arg, modulePrefix) {
				moduleName = m
				rel = arg[len(modulePrefix):]
				break
			}
		}
		if rel == "" {
			moduleName = mainModuleName
			rel = filepath.Join(relCwd, filepath.Clean(arg))
		}

		if strings.HasPrefix(rel, "..") {
			return nil, fnerrors.UserError(nil, "can't refer to packages outside of the module root: %s", rel)
		}

		fsloc := fnfs.Location{
			ModuleName: moduleName,
			RelPath:    rel,
		}
		locations = append(locations, fsloc)
	}

	return locations, nil
}

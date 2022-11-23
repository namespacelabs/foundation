// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fncobra

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/internal/parsing/module"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
)

type Locations struct {
	Locs []fnfs.Location
	Refs []*schema.PackageRef
	Root *parsing.Root
	// Whether the user explicitly specified a list of locations.
	// If true, "All" can be not empty if "DefaultToAllWhenEmpty" is true
	UserSpecified bool
}

type LocationsParser struct {
	locsOut *Locations
	env     *cfg.Context
	opts    ParseLocationsOpts
}

type ParseLocationsOpts struct {
	// Verify that exactly one location is specified.
	RequireSingle bool
	// If true, and no locations are specified, then "workspace.ListSchemas" result is used.
	ReturnAllIfNoneSpecified bool
	// If true, locations parsed as package references
	RequirePackageRef bool
}

func ParseLocations(locsOut *Locations, env *cfg.Context, opts ...ParseLocationsOpts) *LocationsParser {
	return &LocationsParser{
		locsOut: locsOut,
		opts:    MergeParseLocationOpts(opts),
		env:     env,
	}
}

func MergeParseLocationOpts(opts []ParseLocationsOpts) ParseLocationsOpts {
	var merged ParseLocationsOpts
	for _, opt := range opts {
		if opt.ReturnAllIfNoneSpecified {
			merged.ReturnAllIfNoneSpecified = true
		}
		if opt.RequireSingle {
			merged.RequireSingle = true
		}
		if opt.RequirePackageRef {
			merged.RequirePackageRef = true
		}
	}
	return merged
}

func (p *LocationsParser) AddFlags(cmd *cobra.Command) {}

func (p *LocationsParser) Parse(ctx context.Context, args []string) error {
	if p.locsOut == nil {
		return fnerrors.InternalError("locsOut must be set")
	}

	result, err := ParseLocs(ctx, args, p.env, p.opts)
	if err != nil {
		return err
	}

	*p.locsOut = *result

	return nil
}

func ParseLocs(ctx context.Context, args []string, env *cfg.Context, opts ParseLocationsOpts) (*Locations, error) {
	root, err := module.FindRoot(ctx, ".")
	if err != nil {
		return nil, err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	relCwd, err := filepath.Rel(root.Abs(), cwd)
	if err != nil {
		return nil, err
	}

	var once sync.Once
	var previousSchemaList parsing.SchemaList
	var previousErr error

	schemaList := func() (parsing.SchemaList, error) {
		once.Do(func() {
			previousSchemaList, previousErr = parsing.ListSchemas(ctx, *env, root)
		})

		return previousSchemaList, previousErr
	}

	var locs []fnfs.Location
	if opts.ReturnAllIfNoneSpecified && len(args) == 0 {
		schemaList, err := schemaList()
		if err != nil {
			return nil, err
		}

		locs = schemaList.Locations
	} else {
		var err error
		locs, err = locationsFromArgs(ctx, root.Workspace().ModuleName(), root.Workspace().Proto().AllReferencedModules(), relCwd, args, schemaList)
		if err != nil {
			return nil, err
		}
	}

	if opts.RequireSingle && len(locs) != 1 {
		return nil, fnerrors.New("expected exactly one package")
	}

	// convert location to package reference if it is required
	var refs []*schema.PackageRef
	if opts.RequirePackageRef {
		for _, l := range locs {
			ref, err := schema.StrictParsePackageRef(l.String())
			if err != nil {
				return nil, fnerrors.New("cannot parse location as package reference: %s", l.String())
			}
			refs = append(refs, ref)
		}

	}

	return &Locations{
		Root:          root,
		Locs:          locs,
		Refs:          refs,
		UserSpecified: len(args) > 0,
	}, nil
}

func locationsFromArgs(ctx context.Context, mainModuleName string, moduleNames []string, cwd string, args []string, listSchemas func() (parsing.SchemaList, error)) ([]fnfs.Location, error) {
	var locations []fnfs.Location
	for _, arg := range args {
		if filepath.IsAbs(arg) {
			return nil, fnerrors.New("absolute paths are not supported: %s", arg)
		}

		origArg := arg
		expando := false
		if strings.HasSuffix(arg, "/...") {
			expando = true
			arg = strings.TrimSuffix(arg, "/...")
		}

		moduleName, rel := matchModule(moduleNames, arg)
		if moduleName == "" {
			moduleName = mainModuleName
			rel = filepath.Join(cwd, arg)
		}

		fmt.Fprintf(console.Debug(ctx), "location parsing: %s -> moduleName: %q rel: %q expando: %v\n", origArg, moduleName, rel, expando)

		if strings.HasPrefix(rel, "..") {
			return nil, fnerrors.New("can't refer to packages outside of the module root: %s", rel)
		}

		if expando {
			schemas, err := listSchemas()
			if err != nil {
				return nil, err
			}

			for _, p := range schemas.Locations {
				if matchesExpando(p, moduleName, rel) {
					locations = append(locations, p)
				}
			}
		} else {
			fsloc := fnfs.Location{
				ModuleName: moduleName,
				RelPath:    rel,
			}

			locations = append(locations, fsloc)
		}
	}

	return locations, nil
}

func matchModule(moduleNames []string, arg string) (string, string) {
	for _, m := range moduleNames {
		modulePrefix := m + "/"
		if strings.HasPrefix(arg, modulePrefix) {
			return m, arg[len(modulePrefix):]
		}
	}

	return "", arg
}

func matchesExpando(p fnfs.Location, moduleName, rel string) bool {
	if p.ModuleName != moduleName {
		return false
	}

	return p.RelPath == rel || strings.HasPrefix(p.RelPath, rel+"/")
}

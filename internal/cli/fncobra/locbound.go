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
	"namespacelabs.dev/foundation/std/cfg"
)

type Locations struct {
	Locs []fnfs.Location
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
}

func ParseLocations(locsOut *Locations, env *cfg.Context, opts ...ParseLocationsOpts) *LocationsParser {
	var merged ParseLocationsOpts
	for _, opt := range opts {
		if opt.ReturnAllIfNoneSpecified {
			merged.ReturnAllIfNoneSpecified = true
		}
		if opt.RequireSingle {
			merged.RequireSingle = true
		}
	}

	return &LocationsParser{
		locsOut: locsOut,
		opts:    merged,
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

	var once sync.Once
	var previousSchemaList parsing.SchemaList
	var previousErr error

	schemaList := func() (parsing.SchemaList, error) {
		once.Do(func() {
			previousSchemaList, previousErr = parsing.ListSchemas(ctx, *p.env, root)
		})

		return previousSchemaList, previousErr
	}

	var locs []fnfs.Location
	if p.opts.ReturnAllIfNoneSpecified && len(args) == 0 {
		schemaList, err := schemaList()
		if err != nil {
			return err
		}

		locs = schemaList.Locations
	} else {
		var err error
		locs, err = locationsFromArgs(ctx, root.Workspace().ModuleName(), root.Workspace().Proto().AllReferencedModules(), relCwd, args, schemaList)
		if err != nil {
			return err
		}
	}

	if p.opts.RequireSingle && len(locs) != 1 {
		return fnerrors.UserError(nil, "expected exactly one package")
	}

	*p.locsOut = Locations{
		Root:          root,
		Locs:          locs,
		UserSpecified: len(args) > 0,
	}

	return nil
}

func locationsFromArgs(ctx context.Context, mainModuleName string, moduleNames []string, cwd string, args []string, listSchemas func() (parsing.SchemaList, error)) ([]fnfs.Location, error) {
	var locations []fnfs.Location
	for _, arg := range args {
		if filepath.IsAbs(arg) {
			return nil, fnerrors.UserError(nil, "absolute paths are not supported: %s", arg)
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
			return nil, fnerrors.UserError(nil, "can't refer to packages outside of the module root: %s", rel)
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

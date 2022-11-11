// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

// This is about parsing Fn-specific dialect of Cue.
package cuefrontend

import (
	"context"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type impl struct {
	loader          parsing.EarlyPackageLoader
	env             *schema.Environment
	evalctx         *fncue.EvalCtx
	newSyntaxParser NewSyntaxParser
}

type NewSyntaxParser interface {
	ParsePackage(ctx context.Context, partial *fncue.Partial, loc pkggraph.Location) (*pkggraph.Package, error)
}

type cueInjectedScope struct {
	// Injecting schema.Environment as $env so the user can use it without importing.
	// It is temporarily optional since not all commands (that should) accept the --env flag.
	Env *cueEnv `json:"$env"`
}

// Variables that always available for the user in CUE files, without explicit importing.
func InjectedScope(env *schema.Environment) interface{} {
	return &cueInjectedScope{
		Env: &cueEnv{
			Name:      env.Name,
			Runtime:   env.Runtime,
			Purpose:   env.Purpose.String(),
			Ephemeral: env.Ephemeral,
		},
	}
}

func NewFrontend(pl parsing.EarlyPackageLoader, opaqueParser NewSyntaxParser, env *schema.Environment) parsing.Frontend {
	return impl{
		loader:          pl,
		env:             env,
		evalctx:         fncue.NewEvalCtx(WorkspaceLoader{pl}, InjectedScope(env)),
		newSyntaxParser: opaqueParser,
	}
}

func (ft impl) ParsePackage(ctx context.Context, loc pkggraph.Location) (*pkggraph.Package, error) {
	partial, err := parsePackage(ctx, ft.evalctx, ft.loader, loc)
	if err != nil {
		return nil, err
	}

	// Packages in the new syntax don't rely as much on cue features. They're
	// streamlined data definitions without the constraints of json.
	if isNewSyntax(partial) {
		return ft.newSyntaxParser.ParsePackage(ctx, partial, loc)
	}

	v := &partial.CueV

	parsed := &pkggraph.Package{
		Location:       loc,
		PackageSources: partial.Package.Snapshot,
		Parsed:         phase1plan{owner: loc.PackageName, partial: partial, Value: v, Left: partial.Left},
	}

	var count int
	if extension := v.LookupPath("extension"); extension.Exists() {
		if err := parseCueNode(ctx, ft.env, ft.loader, loc, schema.Node_EXTENSION, v, extension, parsed); err != nil {
			return nil, fnerrors.NewWithLocation(loc, "parsing extension: %w", err)
		}
		count++
	}

	if service := v.LookupPath("service"); service.Exists() {
		if err := parseCueNode(ctx, ft.env, ft.loader, loc, schema.Node_SERVICE, v, service, parsed); err != nil {
			return nil, fnerrors.NewWithLocation(loc, "parsing service: %w", err)
		}
		count++
	}

	if server := v.LookupPath("server"); server.Exists() {
		parsedSrv, binaries, err := parseCueServer(ctx, ft.loader, loc, v, server)
		if err != nil {
			return nil, fnerrors.NewWithLocation(loc, "parsing server: %w", err)
		}
		parsed.Server = parsedSrv
		parsed.Binaries = append(parsed.Binaries, binaries...)

		count++
	}

	// Binaries should really be called "OCI Images".
	if binary := v.LookupPath("binary"); binary.Exists() {
		parsedBinary, err := parseCueBinary(ctx, loc, v, binary)
		if err != nil {
			return nil, fnerrors.NewWithLocation(loc, "parsing binary: %w", err)
		}
		parsed.Binaries = append(parsed.Binaries, parsedBinary)
		count++
	}

	if test := v.LookupPath("test"); test.Exists() {
		parsedTest, err := parseCueTest(ctx, loc, v, test)
		if err != nil {
			return nil, fnerrors.NewWithLocation(loc, "parsing test: %w", err)
		}
		parsed.Tests = append(parsed.Tests, parsedTest)
		count++
	}

	if function := v.LookupPath("function"); function.Exists() {
		parsedFunction, err := parseCueFunction(ctx, loc, v, function)
		if err != nil {
			return nil, fnerrors.NewWithLocation(loc, "parsing function: %w", err)
		}
		parsed.ExperimentalFunction = parsedFunction
		count++
	}

	if count > 1 {
		return nil, fnerrors.New("package must only define one of: server, service, extension, binary or test")
	}

	return parsed, nil
}

func isNewSyntax(partial *fncue.Partial) bool {
	if len(partial.CueImports) > 1 {
		// There is at least one import: the file itself.
		return false
	}

	// Detecting the simplified syntax to define opaque servers.
	for _, path := range []string{"server", "resources", "resourceClasses", "providers", "volumes", "secrets", "tests"} {
		if partial.CueV.LookupPath(path).Exists() {
			return true
		}
	}

	return false
}

func (ft impl) GuessPackageType(ctx context.Context, pkg schema.PackageName) (parsing.PackageType, error) {
	firstPass, err := ft.evalctx.EvalPackage(ctx, pkg.String())
	if err != nil {
		return parsing.PackageType_None, err
	}

	topLevels := map[string]parsing.PackageType{
		"service":   parsing.PackageType_Service,
		"server":    parsing.PackageType_Server,
		"extension": parsing.PackageType_Extension,
		"test":      parsing.PackageType_Test,
		"binary":    parsing.PackageType_Binary,
	}
	for k, v := range topLevels {
		if firstPass.LookupPath(k).Exists() {
			return v, nil
		}
	}

	return parsing.PackageType_None, nil
}

func (ft impl) HasNodePackage(ctx context.Context, pkg schema.PackageName) (bool, error) {
	firstPass, err := ft.evalctx.EvalPackage(ctx, pkg.String())
	if err != nil {
		return false, err
	}

	var topLevels = []string{"service", "extension"}
	for _, topLevel := range topLevels {
		if firstPass.LookupPath(topLevel).Exists() {
			return true, nil
		}
	}

	return false, nil
}

type WorkspaceLoader struct {
	PackageLoader parsing.EarlyPackageLoader
}

func (wl WorkspaceLoader) SnapshotDir(ctx context.Context, pkgname schema.PackageName, opts memfs.SnapshotOpts) (*fncue.PackageContents, error) {
	loc, err := wl.PackageLoader.Resolve(ctx, pkgname)
	if err != nil {
		return nil, err
	}

	w, err := wl.PackageLoader.WorkspaceOf(ctx, loc.Module)
	if err != nil {
		return nil, err
	}

	fsys, err := memfs.SnapshotDir(w, loc.Rel(), opts)
	if err != nil {
		return nil, err
	}

	return &fncue.PackageContents{
		ModuleName: loc.Module.ModuleName(),
		RelPath:    loc.Rel(),
		Snapshot:   fsys,
		AbsPath:    loc.Abs(),
	}, nil
}

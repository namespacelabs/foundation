// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

// This is about parsing Fn-specific dialect of Cue.
package cuefrontend

import (
	"context"
	"fmt"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/internal/versions"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

const (
	Version_SyntaxVersionMarker = 54
	syntaxVersionMarker         = "namespaceInternalParserVersion"
	oldSyntaxVersion            = 1
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

type cueEnv struct {
	Name      string            `json:"name"`
	Runtime   string            `json:"runtime"`
	Purpose   string            `json:"purpose"`
	Ephemeral bool              `json:"ephemeral"`
	Labels    map[string]string `json:"labels"`
}

// Variables that always available for the user in CUE files, without explicit importing.
func InjectedScope(env *schema.Environment) *cueInjectedScope {
	labels := map[string]string{}
	for _, lbl := range env.Labels {
		labels[lbl.Name] = lbl.Value
	}

	return &cueInjectedScope{
		Env: &cueEnv{
			Name:      env.Name,
			Runtime:   env.Runtime,
			Purpose:   env.Purpose.String(),
			Ephemeral: env.Ephemeral,
			Labels:    labels,
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

	newSyntax, err := isNewSyntax(ctx, partial, ft.loader)
	if err != nil {
		return nil, err
	}

	// Packages in the new syntax don't rely as much on cue features. They're
	// streamlined data definitions without the constraints of json.
	if newSyntax {
		return ft.newSyntaxParser.ParsePackage(ctx, partial, loc)
	}

	v := &partial.CueV

	parsed, err := ParsePackage(ctx, ft.env, ft.loader, v, loc)
	if err != nil {
		return nil, err
	}

	parsed.PackageSources = partial.Package.Snapshot
	parsed.Parsed = phase1plan{owner: loc.PackageName, partial: partial, Value: v, Left: partial.Left}

	var count int
	if extension := v.LookupPath("extension"); extension.Exists() {
		if err := parseCueNode(ctx, ft.env, ft.loader, loc, schema.Node_EXTENSION, v, extension, parsed); err != nil {
			return nil, fnerrors.NewWithLocation(loc, "failed while parsing extension: %w", err)
		}
		count++
	}

	if service := v.LookupPath("service"); service.Exists() {
		if err := parseCueNode(ctx, ft.env, ft.loader, loc, schema.Node_SERVICE, v, service, parsed); err != nil {
			return nil, fnerrors.NewWithLocation(loc, "failed while parsing service: %w", err)
		}
		count++
	}

	if server := v.LookupPath("server"); server.Exists() {
		parsedSrv, binaries, err := parseCueServer(ctx, ft.loader, loc, v, server)
		if err != nil {
			return nil, fnerrors.NewWithLocation(loc, "failed while parsing server: %w", err)
		}
		parsed.Server = parsedSrv
		parsed.Binaries = append(parsed.Binaries, binaries...)

		count++
	}

	if test := v.LookupPath("test"); test.Exists() {
		parsedTest, err := parsecueTestOld(ctx, loc, v, test)
		if err != nil {
			return nil, fnerrors.NewWithLocation(loc, "parsing test: %w", err)
		}
		parsed.Tests = append(parsed.Tests, parsedTest)
		count++
	}

	if count > 1 {
		return nil, fnerrors.New("package must only define one of: server, service, extension, binary or test")
	}

	return parsed, nil
}

func isNewSyntax(ctx context.Context, partial *fncue.Partial, pl parsing.EarlyPackageLoader) (bool, error) {
	supportsMarker, err := supportsSyntaxVersionMarker(ctx, pl)
	if err != nil {
		return false, err
	}

	if supportsMarker {
		// Detecting the old syntax using a syntax version marker.
		for _, path := range []string{"server", "service", "extension", "configure", "test"} {
			v := partial.CueV.LookupPath(fmt.Sprintf("%s.%s", path, syntaxVersionMarker))
			if !v.Exists() {
				continue
			}

			version, err := v.Val.Int64()
			if err == nil && version == oldSyntaxVersion {
				return false, nil
			}
		}

		return true, nil
	}

	if len(partial.CueImports) > 1 {
		// There is at least one import: the file itself.
		return false, nil
	}

	// Detecting the old syntax.
	for _, path := range []string{"service", "extension", "test"} {
		if partial.CueV.LookupPath(path).Exists() {
			return false, nil
		}
	}

	return true, nil
}

func supportsSyntaxVersionMarker(ctx context.Context, pl parsing.EarlyPackageLoader) (bool, error) {
	pkg, err := pl.Resolve(ctx, "namespacelabs.dev/foundation")
	if err != nil {
		return false, err
	}

	data, err := versions.LoadAtOrDefaults(pkg.Module.ReadOnlyFS(), "internal/versions/versions.json")
	if err != nil {
		return false, fnerrors.InternalError("failed to load namespacelabs.dev/foundation version data: %w", err)
	}

	return data.APIVersion >= Version_SyntaxVersionMarker, nil

}

func (ft impl) GuessPackageType(ctx context.Context, pkg schema.PackageName) (parsing.PackageType, error) {
	firstPass, err := ft.evalctx.EvalPackage(ctx, pkg.String())
	if err != nil {
		return parsing.PackageType_None, err
	}

	topLevels := map[string]parsing.PackageType{
		"service":   parsing.PackageType_Service,
		"server":    parsing.PackageType_Server, // TODO This can be old or new syntax. In new syntax, there can be other primitives in the same package.
		"extension": parsing.PackageType_Extension,
		"test":      parsing.PackageType_Test,
		"binary":    parsing.PackageType_Binary,

		// TODO consider refining.
		"secrets":         parsing.PackageType_NewSyntax,
		"resources":       parsing.PackageType_NewSyntax,
		"resourceClasses": parsing.PackageType_NewSyntax,
		"providers":       parsing.PackageType_NewSyntax,
		"volumes":         parsing.PackageType_NewSyntax,
		"tests":           parsing.PackageType_NewSyntax,
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

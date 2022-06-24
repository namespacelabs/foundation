// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

// This is about parsing Fn-specific dialect of Cue.
package cuefrontend

import (
	"context"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
)

type impl struct {
	loader  workspace.EarlyPackageLoader
	evalctx *fncue.EvalCtx
}

func NewFrontend(pl workspace.EarlyPackageLoader) workspace.Frontend {
	return impl{
		loader:  pl,
		evalctx: fncue.NewEvalCtx(WorkspaceLoader{pl}),
	}
}

func (ft impl) ParsePackage(ctx context.Context, loc workspace.Location, opts workspace.LoadPackageOpts) (*workspace.Package, error) {
	partial, err := parsePackage(ctx, ft.evalctx, ft.loader, loc)
	if err != nil {
		return nil, err
	}

	v := &partial.CueV

	parsed := &workspace.Package{
		Location: loc,
		Parsed:   phase1plan{partial: partial, Value: v, Left: partial.Left},
	}

	var count int
	if extension := v.LookupPath("extension"); extension.Exists() {
		if err := parseCueNode(ctx, ft.loader, loc, schema.Node_EXTENSION, v, extension, parsed, opts); err != nil {
			return nil, fnerrors.Wrapf(loc, err, "parsing extension")
		}
		count++
	}

	if service := v.LookupPath("service"); service.Exists() {
		if err := parseCueNode(ctx, ft.loader, loc, schema.Node_SERVICE, v, service, parsed, opts); err != nil {
			return nil, fnerrors.Wrapf(loc, err, "parsing service")
		}
		count++
	}

	if server := v.LookupPath("server"); server.Exists() {
		parsedSrv, err := parseCueServer(ctx, ft.loader, loc, v, server, parsed, opts)
		if err != nil {
			return nil, fnerrors.Wrapf(loc, err, "parsing server")
		}
		parsed.Server = parsedSrv
		count++
	}
	if binary := v.LookupPath("binary"); binary.Exists() {
		parsedBinary, err := parseCueBinary(ctx, loc, v, binary)
		if err != nil {
			return nil, fnerrors.Wrapf(loc, err, "parsing binary")
		}
		parsed.Binary = parsedBinary
		count++
	}
	if test := v.LookupPath("test"); test.Exists() {
		parsedTest, err := parseCueTest(ctx, loc, v, test)
		if err != nil {
			return nil, fnerrors.Wrapf(loc, err, "parsing test")
		}
		parsed.Test = parsedTest
		count++
	}

	if count > 1 {
		return nil, fnerrors.New("package must only define one of: server, service, extension, binary or test")
	}

	return parsed, nil
}

func (ft impl) GuessPackageType(ctx context.Context, pkg schema.PackageName) (workspace.PackageType, error) {
	firstPass, err := ft.evalctx.EvalPackage(ctx, pkg.String())
	if err != nil {
		return workspace.PackageType_None, err
	}

	topLevels := map[string]workspace.PackageType{
		"service":   workspace.PackageType_Service,
		"server":    workspace.PackageType_Server,
		"extension": workspace.PackageType_Extension,
		"test":      workspace.PackageType_Test,
		"binary":    workspace.PackageType_Binary,
	}
	for k, v := range topLevels {
		if firstPass.LookupPath(k).Exists() {
			return v, nil
		}
	}

	return workspace.PackageType_None, nil
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
	PackageLoader workspace.EarlyPackageLoader
}

func (wl WorkspaceLoader) SnapshotDir(ctx context.Context, pkgname schema.PackageName, opts memfs.SnapshotOpts) (fnfs.Location, string, error) {
	loc, err := wl.PackageLoader.Resolve(ctx, pkgname)
	if err != nil {
		return fnfs.Location{}, "", err
	}

	w, err := wl.PackageLoader.WorkspaceOf(ctx, loc.Module)
	if err != nil {
		return fnfs.Location{}, "", err
	}

	fsys, err := w.SnapshotDir(loc.Rel(), opts)
	if err != nil {
		return fnfs.Location{}, "", err
	}

	return fnfs.Location{
		ModuleName: loc.Module.ModuleName(),
		RelPath:    loc.Rel(),
		FS:         fsys,
	}, loc.Abs(), nil
}

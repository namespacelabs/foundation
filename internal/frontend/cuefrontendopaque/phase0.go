// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cuefrontendopaque

import (
	"context"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
)

type Frontend struct {
	loader  workspace.EarlyPackageLoader
	evalctx *fncue.EvalCtx
}

func NewFrontend(pl workspace.EarlyPackageLoader) *Frontend {
	return &Frontend{
		loader:  pl,
		evalctx: fncue.NewEvalCtx(cuefrontend.WorkspaceLoader{PackageLoader: pl}),
	}
}

func (ft Frontend) ParsePackage(ctx context.Context, partial *fncue.Partial, loc workspace.Location, opts workspace.LoadPackageOpts) (*workspace.Package, error) {
	v := &partial.CueV

	phase1plan := &phase1plan{}
	parsedPkg := &workspace.Package{
		Location: loc,
		Parsed:   phase1plan,
	}

	server := v.LookupPath("server")
	if !server.Exists() {
		return nil, fnerrors.UserError(loc, "Missing server field")
	}

	var parsedVolumes []*schema.Volume
	if volumes := v.LookupPath("volumes"); volumes.Exists() {
		var err error
		parsedVolumes, err = cuefrontend.ParseVolumes(ctx, ft.loader, loc, volumes)
		if err != nil {
			return nil, fnerrors.Wrapf(loc, err, "parsing volumes")
		}
	}

	parsedSrv, startupPlan, err := parseCueServer(ctx, ft.loader, loc, v, server, parsedPkg, parsedVolumes, opts)
	if err != nil {
		return nil, fnerrors.Wrapf(loc, err, "parsing server")
	}
	parsedPkg.Server = parsedSrv
	phase1plan.startupPlan = startupPlan

	return parsedPkg, nil
}

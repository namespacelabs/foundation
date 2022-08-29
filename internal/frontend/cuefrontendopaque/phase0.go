// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cuefrontendopaque

import (
	"context"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
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
	parsed := &workspace.Package{
		Location: loc,
		Parsed:   phase1plan,
	}

	if server := v.LookupPath("server"); server.Exists() {
		parsedSrv, startupPlan, err := parseCueServer(ctx, ft.loader, loc, v, server, parsed, opts)
		if err != nil {
			return nil, fnerrors.Wrapf(loc, err, "parsing server")
		}
		parsed.Server = parsedSrv
		phase1plan.startupPlan = startupPlan
	}

	return parsed, nil
}

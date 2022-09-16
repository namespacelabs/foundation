// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cuefrontendopaque

import (
	"context"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/workspace"
)

func parseResourceRef(ctx context.Context, pl workspace.EarlyPackageLoader, loc pkggraph.Location, pkgRefStr string) (*schema.ResourceInstance, error) {
	pkgRef, err := schema.ParsePackageRef(pkgRefStr)
	if err != nil {
		return nil, err
	}

	pkg, err := pl.LoadByName(ctx, pkgRef.AsPackageName())
	if err != nil {
		return nil, err
	}

	r := pkg.ResourceInstance(pkgRef.Name)
	if r == nil {
		return nil, fnerrors.UserError(loc, "no such resource %q", pkgRef.Name)
	}

	return r, nil
}

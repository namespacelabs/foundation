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

func parseResourceRef(ctx context.Context, pl workspace.EarlyPackageLoader, loc pkggraph.Location, packageRef string) (*schema.PackageRef, error) {
	pkgRef, err := schema.ParsePackageRef(packageRef)
	if err != nil {
		return nil, err
	}

	pkg, err := pl.LoadByName(ctx, pkgRef.AsPackageName())
	if err != nil {
		return nil, err
	}

	r := pkg.LookupResourceInstance(pkgRef.Name)
	if r == nil {
		return nil, fnerrors.UserError(loc, "no such resource %q", pkgRef.Name)
	}

	return pkgRef, nil
}

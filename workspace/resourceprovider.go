// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package workspace

import (
	"context"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

func transformResourceProvider(ctx context.Context, pl EarlyPackageLoader, pp *pkggraph.Package, provider *schema.ResourceProvider) error {
	pkg, err := pl.LoadByName(ctx, provider.ProvidesClass.AsPackageName())
	if err != nil {
		return err
	}

	rc := pkg.ResourceClass(provider.ProvidesClass.Name)
	if rc == nil {
		return fnerrors.UserError(pp.Location, "resource class %q not found in package %q", provider.ProvidesClass.Name, provider.ProvidesClass.PackageName)
	}

	if pp.ProvidedResourceClass(provider.ProvidesClass) != nil {
		// Shouldn't happen since the resource class ref is a map key in CUE.
		return fnerrors.InternalError("resource class %q already provided by this package", provider.ProvidesClass.Canonical())
	}

	pp.ProvidedResourceClasses = append(pp.ProvidedResourceClasses, rc)

	return nil
}

// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package workspace

import (
	"context"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnerrors/multierr"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

func transformResourceProvider(ctx context.Context, pl EarlyPackageLoader, pp *pkggraph.Package, provider *schema.ResourceProvider) error {
	if provider.InitializedWith == nil {
		return fnerrors.UserError(pp.Location, "resource provider requires initializedWith")
	}

	if _, _, err := pkggraph.LoadBinary(ctx, pl, provider.InitializedWith.BinaryRef); err != nil {
		return err
	}

	if provider.GetPrepareWith().GetBinaryRef() != nil {
		if _, _, err := pkggraph.LoadBinary(ctx, pl, provider.PrepareWith.BinaryRef); err != nil {
			return err
		}
	}

	pkg, err := pl.LoadByName(ctx, provider.ProvidesClass.AsPackageName())
	if err != nil {
		return err
	}

	rc := pkg.LookupResourceClass(provider.ProvidesClass.Name)
	if rc == nil {
		return fnerrors.UserError(pp.Location, "resource class %q not found in package %q", provider.ProvidesClass.Name, provider.ProvidesClass.PackageName)
	}

	if len(provider.ResourceInstance) > 0 {
		return fnerrors.UserError(pp.Location, "%s: inline resources not yet supported", provider.ProvidesClass.Canonical())
	}

	// Make sure that all referenced classes and providers are loaded.
	var errs []error
	for _, pkg := range provider.ResourceInstanceFromAvailableClasses {
		_, err := pl.LoadByName(ctx, pkg.AsPackageName())
		errs = append(errs, err)
	}

	for _, pkg := range provider.ResourceInstanceFromAvailableProviders {
		_, err := pl.LoadByName(ctx, pkg.AsPackageName())
		errs = append(errs, err)
	}

	return multierr.New(errs...)
}

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

func transformResourceProvider(ctx context.Context, pl EarlyPackageLoader, pp *pkggraph.Package, provider *schema.ResourceProvider) (*pkggraph.ResourceProvider, error) {
	if provider.InitializedWith == nil {
		return nil, fnerrors.UserError(pp.Location, "resource provider requires initializedWith")
	}

	if _, _, err := pkggraph.LoadBinary(ctx, pl, provider.InitializedWith.BinaryRef); err != nil {
		return nil, err
	}

	if provider.GetPrepareWith().GetBinaryRef() != nil {
		if _, _, err := pkggraph.LoadBinary(ctx, pl, provider.PrepareWith.BinaryRef); err != nil {
			return nil, err
		}
	}

	pkg, err := pl.LoadByName(ctx, provider.ProvidesClass.AsPackageName())
	if err != nil {
		return nil, err
	}

	rc := pkg.LookupResourceClass(provider.ProvidesClass.Name)
	if rc == nil {
		return nil, fnerrors.UserError(pp.Location, "resource class %q not found in package %q", provider.ProvidesClass.Name, provider.ProvidesClass.PackageName)
	}

	rp := pkggraph.ResourceProvider{Spec: provider}

	instances, err := LoadResources(ctx, pl, pp, provider.ResourcePack)
	if err != nil {
		return nil, err
	}

	rp.Resources = instances

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

	if err := multierr.New(errs...); err != nil {
		return nil, err
	}

	return &rp, nil
}

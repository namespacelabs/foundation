// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package parsing

import (
	"context"

	"namespacelabs.dev/foundation/framework/rpcerrors/multierr"
	"namespacelabs.dev/foundation/internal/fnerrors"
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

	var errs []error

	if _, err := pkggraph.LookupResourceClass(ctx, pl, provider.ProvidesClass); err != nil {
		errs = append(errs, err)
	}

	rp := pkggraph.ResourceProvider{Spec: provider}

	for _, input := range provider.ResourceInput {
		if rp.LookupExpected(input.Name) != nil {
			errs = append(errs, fnerrors.BadInputError("resource input %q defined more than once", input.Name.Canonical()))
			continue
		}

		class, err := pkggraph.LookupResourceClass(ctx, pl, input.Class)
		if err != nil {
			errs = append(errs, err)
		} else {
			rp.ResourceInputs = append(rp.ResourceInputs, pkggraph.ExpectedResourceInstance{
				Name:  input.Name,
				Class: *class,
			})
		}
	}

	if instances, err := LoadResources(ctx, pl, pp, provider.ResourcePack); err != nil {
		errs = append(errs, err)
	} else {
		for _, instance := range instances {
			if rp.LookupExpected(instance.ResourceRef) != nil {
				errs = append(errs, fnerrors.BadInputError("%q is both a resource input and a static input", instance.ResourceRef.Name))
			} else {
				rp.Resources = append(rp.Resources, instance)
			}
		}
	}

	// Make sure that all referenced classes and providers are loaded.
	for _, pkg := range provider.ResourceInstanceFromAvailableClasses {
		_, err := pl.LoadByName(ctx, pkg.AsPackageName())
		errs = append(errs, err)
	}

	for _, pkg := range provider.ResourceInstanceFromAvailableProviders {
		_, err := pl.LoadByName(ctx, pkg.AsPackageName())
		errs = append(errs, err)
	}

	if err := multierr.New(errs...); err != nil {
		return nil, fnerrors.Wrap(pp.Location, err)
	}

	return &rp, nil
}

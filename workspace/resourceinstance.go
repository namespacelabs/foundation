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

func transformResourceInstance(ctx context.Context, pl EarlyPackageLoader, pp *pkggraph.Package, r *schema.ResourceInstance) error {
	r.PackageName = string(pp.PackageName())

	if r.Provider == "" {
		return fnerrors.UserError(pp.Location, "missing provider for resource instance %q", r.Name)
	}

	if r.Intent != nil && r.IntentFrom != nil {
		return fnerrors.UserError(pp.Location, "resource instance %q cannot specify both \"intent\" and \"from\"", r.Name)
	}

	if r.IntentFrom != nil {
		if _, _, err := pkggraph.LoadBinary(ctx, pl, r.IntentFrom.BinaryRef); err != nil {
			return err
		}
	}

	classPkg, err := pl.LoadByName(ctx, r.Class.AsPackageName())
	if err != nil {
		return err
	}

	class := classPkg.LookupResourceClass(r.Class.Name)
	if class == nil {
		return fnerrors.UserError(pp.Location, "no such resource class %q", r.Class.Canonical())
	}

	providerPkg, err := pl.LoadByName(ctx, schema.PackageName(r.Provider))
	if err != nil {
		return err
	}

	provider := providerPkg.LookupResourceProvider(r.Class)
	if provider == nil {
		return fnerrors.UserError(pp.Location, "package %q does not a provider for resource class %q", r.Provider, r.Class.Canonical())
	}

	pp.Resources = append(pp.Resources, pkggraph.Resource{
		Spec:            r,
		Class:           *class,
		ProviderPackage: providerPkg,
		Provider:        provider,
	})

	return nil
}

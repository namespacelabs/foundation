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

	providerPkg, err := pl.LoadByName(ctx, schema.PackageName(r.Provider))
	if err != nil {
		return err
	}
	provider := providerPkg.ResourceProvider(r.Class)
	if provider == nil {
		return fnerrors.UserError(pp.Location, "package %q does not a provider for resource class %q", r.Provider, r.Class.Canonical())
	}
	// Keeping RequiredResourceProviders unique.
	if pp.RequiredResourceProvider(r.Class) == nil {
		pp.RequiredResourceProviders = append(pp.RequiredResourceProviders, provider)
	}

	return nil
}

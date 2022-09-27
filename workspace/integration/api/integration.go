// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package api

import (
	"context"
	"sort"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/std/pkggraph"
)

var (
	// Key: kind
	registeredIntegrations = map[string]IntegrationApplier{}
	sortedIntegrationKinds []string
)

// Must be called before ApplyIntegration.
func Register(i IntegrationApplier) {
	registeredIntegrations[i.Kind()] = i
	// Caching a deterministic order of integrations
	sortedIntegrationKinds = append(sortedIntegrationKinds, i.Kind())
	sort.Strings(sortedIntegrationKinds)
}

func ApplyIntegration(ctx context.Context, pkg *pkggraph.Package) error {
	if pkg.Integration == nil {
		return nil
	}

	if i, ok := registeredIntegrations[pkg.Integration.Kind]; ok {
		return i.Apply(ctx, pkg.Integration.Data, pkg)
	} else {
		return fnerrors.UserError(pkg.Location, "unknown integration kind: %s", pkg.Integration)
	}
}

// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package api

import (
	"context"
	"sort"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/std/pkggraph"
)

var (
	// Key: kind
	registeredIntegrations = map[string]Integration{}
	sortedIntegrationKinds []string
)

// Must be called before ParseIntegration.
func Register(i Integration) {
	registeredIntegrations[i.Kind()] = i
	// Caching a deterministic order of integrations
	sortedIntegrationKinds = append(sortedIntegrationKinds, i.Kind())
	sort.Strings(sortedIntegrationKinds)
}

// Mutates "pkg"
func ParseIntegration(ctx context.Context, loc pkggraph.Location, v *fncue.CueV, pkg *pkggraph.Package) error {
	// First checking for the full kind
	if kind := v.LookupPath("kind"); kind.Exists() {
		str, err := kind.Val.String()
		if err != nil {
			return err
		}

		if i, ok := registeredIntegrations[str]; ok {
			return i.Parse(ctx, pkg, v)
		} else {
			return fnerrors.UserError(loc, "unknown integration kind: %s", str)
		}
	}

	// If the kind is not specified, trying the short form, e.g.:
	//   integration: golang {
	//	   pkg: "."
	//   }
	for _, kind := range sortedIntegrationKinds {
		i := registeredIntegrations[kind]
		if shortV := v.LookupPath(i.Shortcut()); shortV.Exists() {
			return i.Parse(ctx, pkg, shortV)
		}
		// Shortest form:
		//  integration: "golang"
		if str, err := v.Val.String(); err == nil && str == i.Shortcut() {
			return i.Parse(ctx, pkg, nil)
		}
	}

	return fnerrors.UserError(loc, "integration is not recognized")
}

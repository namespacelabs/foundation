// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package api

import (
	"context"
	"sort"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/workspace"
)

var (
	// Key: kind
	registeredIntegrations = map[string]IntegrationParser{}
	sortedIntegrationKinds []string
)

// Must be called before ParseIntegration.
func Register(i IntegrationParser) {
	registeredIntegrations[i.Kind()] = i
	// Caching a deterministic order of integrations
	sortedIntegrationKinds = append(sortedIntegrationKinds, i.Kind())
	sort.Strings(sortedIntegrationKinds)
}

type ParsedIntegration struct {
	Kind string
	Data proto.Message
}

func ParseIntegration(ctx context.Context, pl workspace.EarlyPackageLoader, loc pkggraph.Location, v *fncue.CueV) (ParsedIntegration, error) {
	// First checking for the full kind
	if kind := v.LookupPath("kind"); kind.Exists() {
		str, err := kind.Val.String()
		if err != nil {
			return ParsedIntegration{}, err
		}

		if i, ok := registeredIntegrations[str]; ok {
			return parse(ctx, pl, loc, i, v)
		} else {
			return ParsedIntegration{}, fnerrors.UserError(loc, "unknown integration kind: %s", str)
		}
	}

	// If the kind is not specified, trying the short form, e.g.:
	//   integration: golang {
	//	   pkg: "."
	//   }
	for _, kind := range sortedIntegrationKinds {
		i := registeredIntegrations[kind]
		if shortV := v.LookupPath(i.Shortcut()); shortV.Exists() {
			return parse(ctx, pl, loc, i, shortV)
		}
		// Shortest form:
		//  integration: "golang"
		if str, err := v.Val.String(); err == nil && str == i.Shortcut() {
			return parse(ctx, pl, loc, i, nil)
		}
	}

	return ParsedIntegration{}, fnerrors.UserError(loc, "integration is not recognized")
}

func parse(ctx context.Context, pl workspace.EarlyPackageLoader, loc pkggraph.Location, i IntegrationParser, v *fncue.CueV) (ParsedIntegration, error) {
	data, err := i.Parse(ctx, pl, loc, v)
	if err != nil {
		return ParsedIntegration{}, err
	}
	return ParsedIntegration{
		Kind: i.Kind(),
		Data: data,
	}, nil
}

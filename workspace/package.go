// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package workspace

import (
	"context"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/workspace/integration/api"
)

// This function contains frontend-agnostic validation and processing code.
func FinalizePackage(ctx context.Context, env *schema.Environment, pl EarlyPackageLoader, pp *pkggraph.Package, opts LoadPackageOpts) (*pkggraph.Package, error) {
	var err error

	if pp.Integration != nil {
		if err = api.ApplyPackageIntegration(ctx, env, pl, pp); err != nil {
			return nil, err
		}
	}

	if pp.Server != nil {
		pp.Server, err = TransformServer(ctx, pl, pp.Server, pp, opts)
		if err != nil {
			return nil, err
		}
	}

	for _, test := range pp.Tests {
		if err := transformTest(pp.Location, pp.Server, test); err != nil {
			return nil, err
		}
	}

	if pp.Server != nil && pp.Server.RunByDefault {
		test, err := createServerStartupTest(ctx, pl, pp.PackageName())
		if err != nil {
			return nil, fnerrors.Wrapf(pp.Location, err, "creating server startup test")
		}
		pp.Tests = append(pp.Tests, test)
	}

	for _, binary := range pp.Binaries {
		if err := transformBinary(pp.Location, binary); err != nil {
			return nil, err
		}
	}

	if err := transformResourceClasses(ctx, pl, pp); err != nil {
		return nil, err
	}

	for _, provider := range pp.ResourceProvidersSpecs {
		if err := transformResourceProvider(ctx, pl, pp, provider); err != nil {
			return nil, err
		}

		rp := pkggraph.ResourceProvider{Spec: provider}

		for _, resource := range provider.ResourceInstance {
			if parsed, err := LoadResourceInstance(ctx, pl, pp, resource); err != nil {
				return nil, err
			} else {
				rp.InlineResources = append(rp.InlineResources, *parsed)
			}
		}

		pp.ResourceProviders = append(pp.ResourceProviders, rp)
	}

	for _, r := range pp.ResourceInstanceSpecs {
		if parsed, err := LoadResourceInstance(ctx, pl, pp, r); err != nil {
			return nil, err
		} else {
			pp.Resources = append(pp.Resources, *parsed)
		}
	}

	return pp, nil
}

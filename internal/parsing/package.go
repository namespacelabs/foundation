// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package parsing

import (
	"context"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/parsing/integration/api"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

// This function contains frontend-agnostic validation and processing code.
func FinalizePackage(ctx context.Context, env *schema.Environment, pl EarlyPackageLoader, pp *pkggraph.Package) (*pkggraph.Package, error) {
	var err error

	if pp.Integration != nil {
		if err = api.ApplyServerIntegration(ctx, env, pl, pp); err != nil {
			return nil, err
		}
	}

	if pp.Server != nil {
		pp.Server, err = TransformServer(ctx, pl, pp.Server, pp)
		if err != nil {
			return nil, err
		}
	}

	for _, test := range pp.Tests {
		if test.Integration != nil {
			if err = api.ApplyTestIntegration(ctx, env, pl, pp, test); err != nil {
				return nil, err
			}
		}

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
		if rp, err := transformResourceProvider(ctx, pl, pp, provider); err != nil {
			return nil, err
		} else {
			pp.ResourceProviders = append(pp.ResourceProviders, *rp)
		}
	}

	for _, r := range pp.ResourceInstanceSpecs {
		if parsed, err := loadResourceInstance(ctx, pl, pp, r); err != nil {
			return nil, err
		} else {
			pp.Resources = append(pp.Resources, *parsed)
		}
	}

	return pp, nil
}

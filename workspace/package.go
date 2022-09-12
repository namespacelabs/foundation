// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package workspace

import (
	"context"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/std/pkggraph"
)

// This function contains frontend-agnostic validation and processing code.
// The "opaque integration" will be applied here.
func SealPackage(ctx context.Context, pl pkggraph.PackageLoader, pp *pkggraph.Package, opts LoadPackageOpts) (*pkggraph.Package, error) {
	var err error

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

	return pp, nil
}

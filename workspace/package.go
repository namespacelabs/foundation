// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package workspace

import (
	"context"

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

	return pp, nil
}

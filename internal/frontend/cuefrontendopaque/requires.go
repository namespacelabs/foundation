// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cuefrontendopaque

import (
	"context"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/workspace"
)

func parseRequires(ctx context.Context, pl workspace.EarlyPackageLoader, loc pkggraph.Location, v *fncue.CueV) ([]schema.PackageName, error) {
	var bits []schema.PackageName
	if err := v.Val.Decode(&bits); err != nil {
		return nil, err
	}

	for _, p := range bits {
		err := workspace.Ensure(ctx, pl, p)
		if err != nil {
			return nil, fnerrors.Wrapf(loc, err, "loading package %s", p)
		}
	}

	return bits, nil
}

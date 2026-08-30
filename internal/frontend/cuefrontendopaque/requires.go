// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cuefrontendopaque

import (
	"context"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

func parseRequires(ctx context.Context, pl parsing.EarlyPackageLoader, loc pkggraph.Location, v *fncue.CueV) ([]schema.PackageName, error) {
	var bits []schema.PackageName
	if err := v.Val.Decode(&bits); err != nil {
		return nil, err
	}

	for _, p := range bits {
		if err := pl.Ensure(ctx, p); err != nil {
			return nil, fnerrors.NewWithLocation(loc, "loading package %s failed: %w", p, err)
		}
	}

	return bits, nil
}

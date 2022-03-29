// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cuefrontend

import (
	"context"

	"cuelang.org/go/cue"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
)

func parseCueBinary(ctx context.Context, loc workspace.Location, parent, v *fncue.CueV) (*schema.Binary, error) {
	// Ensure all fields are bound.
	if err := v.Val.Validate(cue.Concrete(true)); err != nil {
		return nil, err
	}

	bin := &schema.Binary{}
	if err := v.Val.Decode(bin); err != nil {
		return nil, err
	}

	return workspace.TransformBinary(loc, bin)
}
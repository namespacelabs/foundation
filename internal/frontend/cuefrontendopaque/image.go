// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cuefrontendopaque

import (
	"context"

	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

func ParseImage(ctx context.Context, loc pkggraph.Location, v *fncue.CueV) (*schema.LayeredImageBuildPlan, error) {
	str, err := v.Val.String()
	if err != nil {
		return nil, err
	}

	return &schema.LayeredImageBuildPlan{
		LayerBuildPlan: []*schema.ImageBuildPlan{{ImageId: str}},
	}, nil
}

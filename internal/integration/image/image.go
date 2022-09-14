// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package image

import (
	"context"

	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/internal/integration/api"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

// The "image" integration is special: it is at the top level of the "server".
// Mutates "pkg"
func ParseImageIntegration(ctx context.Context, loc pkggraph.Location, v *fncue.CueV, pkg *pkggraph.Package) error {
	str, err := v.Val.String()
	if err != nil {
		return err
	}

	return api.SetServerBinary(pkg, &schema.LayeredImageBuildPlan{
		LayerBuildPlan: []*schema.ImageBuildPlan{{ImageId: str}},
	})
}

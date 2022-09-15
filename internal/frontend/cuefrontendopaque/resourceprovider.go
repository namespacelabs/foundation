// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cuefrontendopaque

import (
	"context"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type cueResourceProvider struct {
	InitializedWith *cuefrontend.CueInvokeBinary `json:"initializedWith"`

	// TODO: parse resource dependencies.
}

func parseResourceProvider(ctx context.Context, loc pkggraph.Location, key string, v *fncue.CueV) (*schema.ResourceProvider, error) {
	var bits cueResourceProvider
	if err := v.Val.Decode(&bits); err != nil {
		return nil, err
	}

	rcPkgRef, err := schema.ParsePackageRef(key)
	if err != nil {
		return nil, err
	}

	if bits.InitializedWith == nil {
		return nil, fnerrors.UserError(loc, "resource provider requires initializedWith")
	}

	inv, err := bits.InitializedWith.ToFrontend()
	if err != nil {
		return nil, err
	}

	return &schema.ResourceProvider{
		ProvidesClass:  rcPkgRef,
		InitializeWith: inv,
	}, nil
}

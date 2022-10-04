// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cuefrontendopaque

import (
	"context"

	"namespacelabs.dev/foundation/internal/frontend/cuefrontend"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/workspace"
)

type cueResourceProvider struct {
	InitializedWith *cuefrontend.CueInvokeBinary `json:"initializedWith"`
	PrepareWith     *cuefrontend.CueInvokeBinary `json:"prepareWith"`
	Resources       *cuefrontend.ResourceList    `json:"resources"`

	// TODO: parse prepare hook.
}

func parseResourceProvider(ctx context.Context, pl workspace.EarlyPackageLoader, loc pkggraph.Location, key string, v *fncue.CueV) (*schema.ResourceProvider, error) {
	var bits cueResourceProvider
	if err := v.Val.Decode(&bits); err != nil {
		return nil, err
	}

	classRef, err := schema.ParsePackageRef(key)
	if err != nil {
		return nil, err
	}

	initializedWith, err := bits.InitializedWith.ToInvocation()
	if err != nil {
		return nil, err
	}

	rp := &schema.ResourceProvider{
		ProvidesClass:   classRef,
		InitializedWith: initializedWith,
	}

	if bits.Resources != nil {
		pack, err := bits.Resources.ToPack(ctx, pl, loc)
		if err != nil {
			return nil, err
		}
		rp.ResourcePack = pack
	}

	if bits.PrepareWith != nil {
		prepareWith, err := bits.PrepareWith.ToInvocation()
		if err != nil {
			return nil, err
		}
		rp.PrepareWith = prepareWith
	}

	return rp, nil
}

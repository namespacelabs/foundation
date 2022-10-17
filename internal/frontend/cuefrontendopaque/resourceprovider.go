// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cuefrontendopaque

import (
	"context"

	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/internal/fnerrors/multierr"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type cueResourceProvider struct {
	InitializedWith *cuefrontend.CueInvokeBinary `json:"initializedWith"`
	PrepareWith     *cuefrontend.CueInvokeBinary `json:"prepareWith"`
	Resources       *cuefrontend.ResourceList    `json:"resources"`
	ResourceInputs  map[string]string            `json:"inputs"` // Key: name, Value: serialized class ref
	// TODO: parse prepare hook.
}

func parseResourceProvider(ctx context.Context, pl parsing.EarlyPackageLoader, loc pkggraph.Location, key string, v *fncue.CueV) (*schema.ResourceProvider, error) {
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

	var errs []error
	for key, value := range bits.ResourceInputs {
		class, err := schema.ParsePackageRef(value)
		if err != nil {
			errs = append(errs, err)
		} else {
			rp.ResourceInput = append(rp.ResourceInput, &schema.ResourceProvider_ResourceInput{
				Name:  schema.MakePackageRef(loc.PackageName, key),
				Class: class,
			})
		}
	}

	if err := multierr.New(errs...); err != nil {
		return nil, err
	}

	slices.SortFunc(rp.ResourceInput, func(a, b *schema.ResourceProvider_ResourceInput) bool {
		x := a.Name.Compare(b.Name)
		if x == 0 {
			return a.Class.Compare(b.Class) < 0
		}
		return x < 0
	})

	if bits.PrepareWith != nil {
		prepareWith, err := bits.PrepareWith.ToInvocation()
		if err != nil {
			return nil, err
		}
		rp.PrepareWith = prepareWith
	}

	return rp, nil
}

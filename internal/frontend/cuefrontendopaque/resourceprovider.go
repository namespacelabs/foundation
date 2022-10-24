// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cuefrontendopaque

import (
	"context"

	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/framework/rpcerrors/multierr"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend/binary"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type cueResourceProvider struct {
	ResourceInputs map[string]string `json:"inputs"` // Key: name, Value: serialized class ref
	// TODO: parse prepare hook.
}

func parseResourceProvider(ctx context.Context, env *schema.Environment, pl parsing.EarlyPackageLoader, pkg *pkggraph.Package, key string, v *fncue.CueV) (*schema.ResourceProvider, error) {
	var bits cueResourceProvider
	if err := v.Val.Decode(&bits); err != nil {
		return nil, err
	}

	classRef, err := schema.ParsePackageRef(pkg.PackageName(), key)
	if err != nil {
		return nil, err
	}

	initializedWithInvocation, err := binary.ParseBinaryInvocationField(ctx, env, pl, pkg, "genb-res-init-"+key /* binaryName */, "initializedWith" /* cuePath */, v)
	if err != nil {
		return nil, err
	}

	rp := &schema.ResourceProvider{
		ProvidesClass:   classRef,
		InitializedWith: initializedWithInvocation,
	}

	if resources := v.LookupPath("resources"); resources.Exists() {
		resourceList, err := cuefrontend.ParseResourceList(resources)
		if err != nil {
			return nil, fnerrors.Wrapf(pkg.Location, err, "parsing resources")
		}

		pack, err := resourceList.ToPack(ctx, env, pl, pkg)
		if err != nil {
			return nil, err
		}

		rp.ResourcePack = pack
	}

	var errs []error
	for key, value := range bits.ResourceInputs {
		class, err := schema.ParsePackageRef(pkg.PackageName(), value)
		if err != nil {
			errs = append(errs, err)
		} else {
			rp.ResourceInput = append(rp.ResourceInput, &schema.ResourceProvider_ResourceInput{
				Name:  schema.MakePackageRef(pkg.PackageName(), key),
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

	rp.PrepareWith, err = binary.ParseBinaryInvocationField(ctx, env, pl, pkg, "genb-res-prep-"+key /* binaryName */, "prepareWith" /* cuePath */, v)
	if err != nil {
		return nil, err
	}

	return rp, nil
}

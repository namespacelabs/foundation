// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cuefrontend

import (
	"context"
	"strings"

	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/framework/rpcerrors/multierr"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend/binary"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type cueResourceProvider struct {
	ResourceInputs   map[string]string `json:"inputs"`   // Key: name, Value: class reference (package ref)
	ResourceDefaults map[string]string `json:"defaults"` // Key: name, Value: resource reference (package ref)
	Intent           *cueResourceType  `json:"intent,omitempty"`

	AvailableClasses  []string `json:"availableClasses"`
	AvailablePackages []string `json:"availablePackages"`

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

	resourcesFrom, err := binary.ParseBinaryInvocationField(ctx, env, pl, pkg, "genb-res-resfrom-"+key /* binaryName */, "resourcesFrom" /* cuePath */, v)
	if err != nil {
		return nil, err
	}

	rp := &schema.ResourceProvider{
		PackageName:     pkg.PackageName().String(),
		ProvidesClass:   classRef,
		InitializedWith: initializedWithInvocation,
		ResourcesFrom:   resourcesFrom,
	}

	for _, x := range bits.AvailableClasses {
		ref, err := schema.ParsePackageRef(pkg.PackageName(), x)
		if err != nil {
			return nil, err
		}
		rp.AvailableClasses = append(rp.AvailableClasses, ref)
	}

	rp.AvailablePackages = bits.AvailablePackages

	if bits.Intent != nil {
		rp.IntentType = parseResourceType(bits.Intent)
	}

	if resources := v.LookupPath("resources"); resources.Exists() {
		resourceList, err := ParseResourceList(resources)
		if err != nil {
			return nil, fnerrors.NewWithLocation(pkg.Location, "parsing resources failed: %w", err)
		}

		pack, err := resourceList.ToPack(ctx, env, pl, pkg)
		if err != nil {
			return nil, err
		}

		rp.ResourcePack = pack
	}

	var errs []error
	unusedDefaults := maps.Clone(bits.ResourceDefaults)
	for key, value := range bits.ResourceInputs {
		class, err := schema.ParsePackageRef(pkg.PackageName(), value)
		if err != nil {
			errs = append(errs, err)
		} else {
			input := &schema.ResourceProvider_ResourceInput{
				Name:  schema.MakePackageRef(pkg.PackageName(), key),
				Class: class,
			}

			if def, ok := bits.ResourceDefaults[key]; ok {
				parsed, err := schema.ParsePackageRef(pkg.PackageName(), def)
				if err != nil {
					errs = append(errs, err)
				} else {
					input.DefaultResource = parsed
				}
			}

			rp.ResourceInput = append(rp.ResourceInput, input)

			delete(unusedDefaults, key)
		}
	}

	if len(unusedDefaults) > 0 {
		errs = append(errs, fnerrors.New("the following defaults are unused: %s", strings.Join(maps.Keys(unusedDefaults), ", ")))
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

// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cuefrontend

import (
	"context"
	"fmt"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/internal/parsing/invariants"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

// Needs to be consistent with JSON names of cueResourceClass fields.
var resourceClassFields = []string{"intent", "produces", "defaultProvider", "description"}

type cueResourceClass struct {
	Intent          *cueResourceType `json:"intent"`
	Produces        *cueResourceType `json:"produces"`
	DefaultProvider string           `json:"defaultProvider"`
	Description     string           `json:"description"`
}

type cueResourceType struct {
	Type    string `json:"type"`
	Source  string `json:"source"`
	Package string `json:"package"`
}

func parseResourceClass(ctx context.Context, pl parsing.EarlyPackageLoader, loc pkggraph.Location, name string, v *fncue.CueV) (*schema.ResourceClass, error) {
	if err := ValidateNoExtraFields(loc, fmt.Sprintf("resource class %q:", name) /* messagePrefix */, v, resourceClassFields); err != nil {
		return nil, err
	}

	var bits cueResourceClass
	if err := v.Val.Decode(&bits); err != nil {
		return nil, err
	}

	if bits.Produces == nil {
		return nil, fnerrors.NewWithLocation(loc, "resource class %q must specify the provided type", name)
	}

	if bits.Intent != nil {
		return nil, fnerrors.NewWithLocation(loc, "Resource class %q may not specify an intent. Please specify the intent in the resource provider instead.", name)
	}

	instanceType, err := parseResourceType(ctx, pl, loc, bits.Produces)
	if err != nil {
		return nil, fnerrors.AttachLocation(loc, err)
	}

	return &schema.ResourceClass{
		Name:            name,
		InstanceType:    instanceType,
		DefaultProvider: bits.DefaultProvider,
		Description:     bits.Description,
	}, nil
}

func parseResourceType(ctx context.Context, pl parsing.EarlyPackageLoader, loc pkggraph.Location, t *cueResourceType) (*schema.ResourceType, error) {
	if t == nil {
		return nil, nil
	}

	rt := &schema.ResourceType{
		ProtoType:   t.Type,
		ProtoSource: t.Source,
	}

	if t.Package == "" {
		rt.ProtoPackage = loc.PackageName.String()
	} else {
		rt.ProtoPackage = t.Package

		target := schema.PackageName(t.Package)
		if err := invariants.EnsurePackageLoaded(ctx, pl, loc.PackageName, target); err != nil {
			return nil, err
		}
	}

	return rt, nil
}

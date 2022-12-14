// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cuefrontend

import (
	"context"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type cueResourceClass struct {
	Intent          *cueResourceType `json:"intent"`
	Produces        *cueResourceType `json:"produces"`
	DefaultProvider string           `json:"defaultProvider"`
	Description     string           `json:"description"`
}

type cueResourceType struct {
	Type   string `json:"type"`
	Source string `json:"source"`
}

func parseResourceClass(ctx context.Context, loc pkggraph.Location, name string, v *fncue.CueV) (*schema.ResourceClass, error) {
	var bits cueResourceClass
	if err := v.Val.Decode(&bits); err != nil {
		return nil, err
	}

	if bits.Produces == nil {
		return nil, fnerrors.NewWithLocation(loc, "resource class %q must specify the provided type", name)
	}

	return &schema.ResourceClass{
		Name:            name,
		IntentType:      parseResourceType(bits.Intent),
		InstanceType:    parseResourceType(bits.Produces),
		DefaultProvider: bits.DefaultProvider,
		Description:     bits.Description,
	}, nil
}

func parseResourceType(t *cueResourceType) *schema.ResourceType {
	if t == nil {
		return nil
	}

	return &schema.ResourceType{
		ProtoType:   t.Type,
		ProtoSource: t.Source,
	}
}

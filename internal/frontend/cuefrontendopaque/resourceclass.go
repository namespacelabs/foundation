// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cuefrontendopaque

import (
	"context"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type cueResourceClass struct {
	Intent   *cueResourceClassType `json:"intent"`
	Produces *cueResourceClassType `json:"produces"`
}

type cueResourceClassType struct {
	Type   string `json:"type"`
	Source string `json:"source"`
}

func parseResourceClass(ctx context.Context, loc pkggraph.Location, name string, v *fncue.CueV) (*schema.ResourceClass, error) {
	var bits cueResourceClass
	if err := v.Val.Decode(&bits); err != nil {
		return nil, err
	}

	if bits.Intent == nil {
		return nil, fnerrors.UserError(loc, "resource class %q must specify an intent", name)
	}

	if bits.Produces == nil {
		return nil, fnerrors.UserError(loc, "resource class %q must specify the provided type", name)
	}

	return &schema.ResourceClass{
		Name:         name,
		IntentType:   parseResourceClassType(bits.Intent),
		InstanceType: parseResourceClassType(bits.Produces),
	}, nil
}

func parseResourceClassType(t *cueResourceClassType) *schema.ResourceClass_Type {
	return &schema.ResourceClass_Type{
		ProtoType:   t.Type,
		ProtoSource: t.Source,
	}
}

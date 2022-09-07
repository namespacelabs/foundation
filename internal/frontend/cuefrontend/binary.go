// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cuefrontend

import (
	"context"

	"cuelang.org/go/cue"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/workspace"
)

type cueBinary struct {
	Name      string                        `json:"name,omitempty"`
	Config    *schema.BinaryConfig          `json:"config,omitempty"`
	From      *schema.ImageBuildPlan        `json:"from,omitempty"`
	BuildPlan *schema.LayeredImageBuildPlan `json:"build_plan,omitempty"`
}

func parseCueBinary(ctx context.Context, loc pkggraph.Location, parent, v *fncue.CueV) (*schema.Binary, error) {
	// Ensure all fields are bound.
	if err := v.Val.Validate(cue.Concrete(true)); err != nil {
		return nil, err
	}

	var srcBin cueBinary
	if err := v.Val.Decode(&srcBin); err != nil {
		return nil, err
	}

	bin := &schema.Binary{
		Name:      srcBin.Name,
		Config:    srcBin.Config,
		BuildPlan: srcBin.BuildPlan,
	}

	if srcBin.From != nil {
		if srcBin.BuildPlan != nil {
			return nil, fnerrors.UserError(loc, "from and build_plan are exclusive -- only one can be set")
		}

		bin.BuildPlan = &schema.LayeredImageBuildPlan{
			LayerBuildPlan: []*schema.ImageBuildPlan{srcBin.From},
		}
	}

	if err := workspace.TransformBinary(loc, bin); err != nil {
		return nil, err
	}

	return bin, nil
}

func parseCueFunction(ctx context.Context, loc pkggraph.Location, parent, v *fncue.CueV) (*schema.ExperimentalFunction, error) {
	// Ensure all fields are bound.
	if err := v.Val.Validate(cue.Concrete(true)); err != nil {
		return nil, err
	}

	function := &schema.ExperimentalFunction{}
	if err := v.Val.Decode(function); err != nil {
		return nil, err
	}

	if err := workspace.TransformFunction(loc, function); err != nil {
		return nil, err
	}

	return function, nil
}

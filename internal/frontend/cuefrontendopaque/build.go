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

const (
	serverKindDockerfile = "namespace.so/from-dockerfile"
	serverKindGo         = "namespace.so/from-go"
)

type cueBuild struct {
	cueBuildDocker

	With string `json:"with"`

	// Shortcuts
	Docker *cueBuildDocker `json:"docker"`
}

type cueBuildDocker struct {
	Dockerfile string `json:"dockerfile"`
}

type parsedCueBuild struct {
	binaryRef    *schema.PackageRef
	inlineBinary *schema.Binary
}

// Parses the "build" definition.
func parseCueBuild(ctx context.Context, name string, loc pkggraph.Location, v *fncue.CueV) (*parsedCueBuild, error) {
	var bits cueBuild
	if err := v.Val.Decode(&bits); err != nil {
		return nil, err
	}

	// Parsing shortcuts
	if bits.With == "" {
		if bits.Docker != nil {
			bits.cueBuildDocker = *bits.Docker
			bits.With = serverKindDockerfile
		}
	}

	out := &parsedCueBuild{}

	switch bits.With {
	case serverKindDockerfile:
		out.inlineBinary = &schema.Binary{
			Name: name,
			BuildPlan: &schema.LayeredImageBuildPlan{
				LayerBuildPlan: []*schema.ImageBuildPlan{
					{Dockerfile: bits.Dockerfile},
				},
			},
		}
		out.binaryRef = schema.MakePackageRef(loc.PackageName, name)
	default:
		return nil, fnerrors.UserError(loc, "unsupported builder %q", bits.With)
	}

	return out, nil
}

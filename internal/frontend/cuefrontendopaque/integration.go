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

type cueIntegration struct {
	cueIntegrationDocker

	Kind string `json:"kind"`

	// Shortcuts
	Docker *cueIntegrationDocker `json:"docker"`
}

type cueIntegrationDocker struct {
	Dockerfile string `json:"dockerfile"`
}

func parseIntegration(ctx context.Context, loc pkggraph.Location, v *fncue.CueV) (*schema.Integration, error) {
	var bits cueIntegration
	if err := v.Val.Decode(&bits); err != nil {
		return nil, err
	}

	// Parsing shortcuts
	if bits.Kind == "" {
		if bits.Docker != nil {
			bits.cueIntegrationDocker = *bits.Docker
			bits.Kind = serverKindDockerfile
		}
	}

	switch bits.Kind {
	case serverKindDockerfile:
		return &schema.Integration{
			Kind:       bits.Kind,
			Dockerfile: bits.Dockerfile,
		}, nil
	default:
		return nil, fnerrors.UserError(loc, "unsupported integration kind %q", bits.Kind)
	}
}

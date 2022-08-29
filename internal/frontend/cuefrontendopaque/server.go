// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cuefrontendopaque

import (
	"context"

	"cuelang.org/go/cue"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
)

type cueServer struct {
	Name        string         `json:"name"`
	Integration cueIntegration `json:"integration"`

	Args *cuefrontend.ArgsListOrMap `json:"args"`
	Env  map[string]string          `json:"env"`
}

type cueIntegration struct {
	Kind       string `json:"kind"`
	Dockerfile string `json:"dockerfile"`
}

func parseCueServer(ctx context.Context, pl workspace.EarlyPackageLoader, loc workspace.Location, parent, v *fncue.CueV, pp *workspace.Package, opts workspace.LoadPackageOpts) (*schema.Server, *schema.StartupPlan, error) {
	// Ensure all fields are bound.
	if err := v.Val.Validate(cue.Concrete(true)); err != nil {
		return nil, nil, err
	}

	var bits cueServer
	if err := v.Val.Decode(&bits); err != nil {
		return nil, nil, err
	}

	out := &schema.Server{}
	out.Id = bits.Name
	out.Name = bits.Name

	switch bits.Integration.Kind {
	case "namespace.so/from-dockerfile":
		out.Integration = &schema.Server_Integration{
			Kind:       bits.Integration.Kind,
			Dockerfile: bits.Integration.Dockerfile,
		}
	default:
		return nil, nil, fnerrors.UserError(loc, "unsupported integration kind %q", bits.Integration.Kind)
	}

	startupPlan := &schema.StartupPlan{
		Env:  bits.Env,
		Args: bits.Args.Parsed(),
	}

	server, err := workspace.TransformOpaqueServer(ctx, pl, loc, out, pp, opts)
	return server, startupPlan, err
}

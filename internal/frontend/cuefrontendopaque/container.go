// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cuefrontendopaque

import (
	"context"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/workspace"
)

const (
	serverKindDockerfile = "namespace.so/from-dockerfile"
)

type cueContainer struct {
	Integration cueIntegration `json:"integration"`

	Args *cuefrontend.ArgsListOrMap `json:"args"`
	Env  map[string]string          `json:"env"`

	Services map[string]cueService `json:"services"`
}

type cueIntegration struct {
	Kind       string `json:"kind"`
	Dockerfile string `json:"dockerfile"`
}

type parsedCueContainer struct {
	container      *schema.SidecarContainer
	inlineVolumes  []*schema.Volume
	inlineBinaries []*schema.Binary
}

// TODO: make it common for the main "server" container and sidecars.
func parseCueContainer(ctx context.Context, pl workspace.EarlyPackageLoader, name string, loc pkggraph.Location, v *fncue.CueV) (*parsedCueContainer, error) {
	var bits cueContainer
	if err := v.Val.Decode(&bits); err != nil {
		return nil, err
	}

	out := &parsedCueContainer{
		container: &schema.SidecarContainer{
			Name: name,
			Env:  bits.Env,
			Args: bits.Args.Parsed(),
		},
	}

	switch bits.Integration.Kind {
	case serverKindDockerfile:
		out.inlineBinaries = append(out.inlineBinaries, &schema.Binary{
			Name: name,
			From: &schema.ImageBuildPlan{
				Dockerfile: bits.Integration.Dockerfile,
			},
		})
		out.container.BinaryRef = schema.MakePackageRef(loc.PackageName, name)
	default:
		return nil, fnerrors.UserError(loc, "unsupported integration kind %q", bits.Integration.Kind)
	}

	if mounts := v.LookupPath("mounts"); mounts.Exists() {
		var err error
		out.container.Mounts, out.inlineVolumes, err = cuefrontend.ParseMounts(ctx, pl, loc, mounts)
		if err != nil {
			return nil, fnerrors.Wrapf(loc, err, "parsing volumes")
		}
	}

	return out, nil
}

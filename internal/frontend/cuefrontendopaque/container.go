// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cuefrontendopaque

import (
	"context"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend"
	integrationparsing "namespacelabs.dev/foundation/internal/frontend/cuefrontend/integration/api"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/workspace"
	integrationapplying "namespacelabs.dev/foundation/workspace/integration/api"
)

type cueContainer struct {
	Args *cuefrontend.ArgsListOrMap `json:"args"`
	Env  map[string]string          `json:"env"`
}

type parsedCueContainer struct {
	container      *schema.SidecarContainer
	inlineVolumes  []*schema.Volume
	inlineBinaries []*schema.Binary
}

// TODO: make it common for the main "server" container and sidecars.
func parseCueContainer(ctx context.Context, pl workspace.EarlyPackageLoader, pkg *pkggraph.Package, name string, loc pkggraph.Location, v *fncue.CueV) (*parsedCueContainer, error) {
	var bits cueContainer
	if err := v.Val.Decode(&bits); err != nil {
		return nil, err
	}

	out := &parsedCueContainer{
		container: &schema.SidecarContainer{
			Name: name,
			Args: bits.Args.Parsed(),
		},
	}

	for k, v := range bits.Env {
		out.container.Env = append(out.container.Env, &schema.BinaryConfig_EnvEntry{
			Name: k, Value: v,
		})
	}

	if mounts := v.LookupPath("mounts"); mounts.Exists() {
		var err error
		out.container.Mounts, out.inlineVolumes, err = cuefrontend.ParseMounts(ctx, pl, loc, mounts)
		if err != nil {
			return nil, fnerrors.Wrapf(loc, err, "parsing volumes")
		}
	}

	if image := v.LookupPath("image"); image.Exists() {
		bin, err := ParseImage(ctx, loc, image)
		if err != nil {
			return nil, err
		}

		inlineBinary := &schema.Binary{
			Name:      name,
			BuildPlan: bin,
		}
		out.container.BinaryRef = schema.MakePackageRef(loc.PackageName, name)
		out.inlineBinaries = append(out.inlineBinaries, inlineBinary)
	}

	if build := v.LookupPath("build"); build.Exists() {
		if out.container.BinaryRef != nil {
			return nil, fnerrors.UserError(loc, "cannot specify both 'build' and 'image'")
		}

		integration, err := integrationparsing.BuildParser.ParseEntity(ctx, pl, loc, build)
		if err != nil {
			return nil, err
		}

		binaryRef, err := integrationapplying.GenerateBinaryAndAddToPackage(ctx, pl, pkg, name, integration.Data)
		if err != nil {
			return nil, err
		}
		out.container.BinaryRef = binaryRef
	}

	if out.container.BinaryRef == nil {
		return nil, fnerrors.UserError(loc, "missing 'build' or 'image' definition")
	}

	return out, nil
}

// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package docker

import (
	"context"
	"io/fs"

	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/build/buildkit"
	"namespacelabs.dev/foundation/integrations"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/workspace/compute"
)

func Register() {
	integrations.RegisterBuildIntegration(integrationKind, buildImpl{})
}

type buildImpl struct {
}

func (buildImpl) PrepareBuild(ctx context.Context, server provision.Server) (build.Spec, error) {
	loc := server.Location
	fsys, err := compute.GetValue(ctx, loc.Module.VersionedFS(loc.Rel(), false))
	if err != nil {
		return nil, fnerrors.Wrap(loc, err)
	}

	config := server.Package.Server.Integration
	contents, err := fs.ReadFile(fsys.FS(), config.Dockerfile)
	if err != nil {
		return nil, fnerrors.Wrapf(loc, err, "failed to load Dockerfile")
	}

	// XXX consistency: we've already loaded the workspace contents, ideally we'd use those.
	spec, err := buildkit.DockerfileBuild(buildkit.LocalContents{
		Module: loc.Module, Path: loc.Rel(),
	}, contents)
	if err != nil {
		return nil, fnerrors.Wrap(loc, err)
	}

	return spec, nil
}

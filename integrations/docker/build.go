// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package docker

import (
	"context"

	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/build/buildkit"
	"namespacelabs.dev/foundation/integrations/shared"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

func Register() {
	shared.RegisterBuildIntegration(integrationKind, buildImpl{})
}

type buildImpl struct {
}

func (buildImpl) PrepareBuild(ctx context.Context, loc pkggraph.Location, integration *schema.Integration, observeChanges bool) (build.Spec, error) {
	spec, err := buildkit.DockerfileBuild(loc.Rel(), integration.Dockerfile)
	if err != nil {
		return nil, fnerrors.Wrap(loc, err)
	}

	return spec, nil
}

func (buildImpl) PrepareRun(ctx context.Context, server provision.Server, run *runtime.ServerRunOpts) error {
	return nil
}

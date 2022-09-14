// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package docker

import (
	"context"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/internal/integration/api"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type DockerIntegration struct {
}

type cueIntegrationDocker struct {
	Dockerfile string `json:"dockerfile"`
}

func (i *DockerIntegration) Kind() string {
	return "namespace.so/from-dockerfile"
}

func (i *DockerIntegration) Shortcut() string {
	return "docker"
}

func (i *DockerIntegration) Parse(ctx context.Context, pkg *pkggraph.Package, v *fncue.CueV) error {
	var bits cueIntegrationDocker
	if err := v.Val.Decode(&bits); err != nil {
		return err
	}

	if bits.Dockerfile == "" {
		return fnerrors.UserError(pkg.Location, "docker integration requires dockerfile")
	}

	return api.SetServerBinary(pkg, &schema.LayeredImageBuildPlan{
		LayerBuildPlan: []*schema.ImageBuildPlan{{Dockerfile: bits.Dockerfile}},
	}, nil)
}

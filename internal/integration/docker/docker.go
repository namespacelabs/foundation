// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package docker

import (
	"context"
	"io/fs"
	"path/filepath"

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
	if v != nil {
		if err := v.Val.Decode(&bits); err != nil {
			return err
		}
	}

	dockerfile := bits.Dockerfile
	if dockerfile == "" {
		dockerfile = "Dockerfile"
	}

	if _, err := fs.Stat(pkg.Location.Module.ReadOnlyFS(), filepath.Join(pkg.Location.Rel(), dockerfile)); err != nil {
		return fnerrors.Wrapf(pkg.Location, err, "could not find %q file, please verify that the specified dockerfile path is correct", dockerfile)
	}

	return api.SetServerBinary(pkg, &schema.LayeredImageBuildPlan{
		LayerBuildPlan: []*schema.ImageBuildPlan{{Dockerfile: dockerfile}},
	}, nil)
}

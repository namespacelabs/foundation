// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package docker

import (
	"context"
	"io/fs"
	"path/filepath"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/workspace/integration/api"
)

func Apply(ctx context.Context, data *schema.DockerIntegration, pkg *pkggraph.Package) error {
	if pkg.Server == nil {
		// Can't happen with the current syntax.
		return fnerrors.UserError(pkg.Location, "docker integration requires a server")
	}

	dockerfile := data.Dockerfile
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

// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package dockerfile

import (
	"context"
	"io/fs"
	"path/filepath"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/parsing/integration/api"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

func Register() {
	api.RegisterIntegration[*schema.DockerfileIntegration, *schema.DockerfileIntegration](impl{})
}

type impl struct {
	api.BinaryTestIntegration[*schema.DockerfileIntegration]
}

func (impl) ApplyToServer(ctx context.Context, env *schema.Environment, pl pkggraph.PackageLoader, pkg *pkggraph.Package, data *schema.DockerfileIntegration) error {
	if pkg.Server == nil {
		// Can't happen with the current syntax.
		return fnerrors.UserError(pkg.Location, "docker integration requires a server")
	}

	binaryRef, err := api.GenerateBinaryAndAddToPackage(ctx, env, pl, pkg, pkg.Server.Name, data)
	if err != nil {
		return err
	}

	return api.SetServerBinaryRef(pkg, binaryRef)
}

func (impl) CreateBinary(ctx context.Context, env *schema.Environment, pl pkggraph.PackageLoader, loc pkggraph.Location, data *schema.DockerfileIntegration) (*schema.Binary, error) {
	dockerfile := data.Src
	if dockerfile == "" {
		dockerfile = "Dockerfile"
	}

	if _, err := fs.Stat(loc.Module.ReadOnlyFS(), filepath.Join(loc.Rel(), dockerfile)); err != nil {
		return nil, fnerrors.Wrapf(loc, err, "could not find %q file, please verify that the specified dockerfile path is correct", dockerfile)
	}

	return &schema.Binary{
		BuildPlan: &schema.LayeredImageBuildPlan{
			LayerBuildPlan: []*schema.ImageBuildPlan{{Dockerfile: dockerfile}},
		},
	}, nil
}

// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package shellscript

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
	api.RegisterIntegration[*schema.ShellScriptIntegration, *schema.ShellScriptIntegration](impl{})
}

type impl struct {
	api.DefaultBinaryTestIntegration[*schema.ShellScriptIntegration]
}

func (impl) ApplyToServer(ctx context.Context, env *schema.Environment, pl pkggraph.PackageLoader, pkg *pkggraph.Package, data *schema.ShellScriptIntegration) error {
	return fnerrors.UserError(pkg.Location, "shellscript integration is not supported on the server")
}

func (impl) CreateBinary(ctx context.Context, env *schema.Environment, pl pkggraph.PackageLoader, loc pkggraph.Location, data *schema.ShellScriptIntegration) (*schema.Binary, error) {
	if data.Entrypoint == "" {
		return nil, fnerrors.UserError(loc, "missing required field `entrypoint`")
	}

	entrypoint := filepath.Clean(data.Entrypoint)

	if _, err := fs.Stat(loc.Module.ReadOnlyFS(), filepath.Join(loc.Rel(), entrypoint)); err != nil {
		return nil, fnerrors.Wrapf(loc, err, "could not find %q file, please verify that the specified script path is correct", data.Entrypoint)
	}

	required := []string{"bash", "curl"}
	required = append(required, data.RequiredPackages...)

	return &schema.Binary{
		BuildPlan: &schema.LayeredImageBuildPlan{
			LayerBuildPlan: []*schema.ImageBuildPlan{{
				AlpineBuild: &schema.ImageBuildPlan_AlpineBuild{Package: required},
			}, {
				SnapshotFiles: []string{entrypoint},
			}},
		},
		Config: &schema.BinaryConfig{
			WorkingDir: "/",
			// "./" is important to resolve the entrypoint script relative to the working directory.
			Command: []string{"./" + entrypoint},
		},
	}, nil
}

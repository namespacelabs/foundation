// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package integrations

import (
	"context"
	"path/filepath"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/parsing/integration/api"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

func Register() {
	api.RegisterIntegration[*schema.GoIntegration, *schema.GoIntegration](impl{})
}

type impl struct {
	api.DefaultBinaryTestIntegration[*schema.GoIntegration]
}

func (i impl) ApplyToServer(ctx context.Context, env *schema.Environment, pl pkggraph.PackageLoader, pkg *pkggraph.Package, data *schema.GoIntegration) error {
	if pkg.Server == nil {
		// Can't happen with the current syntax.
		return fnerrors.NewWithLocation(pkg.Location, "go integration requires a server")
	}

	bin, err := i.CreateBinary(ctx, env, pl, pkg.Location, data)
	if err != nil {
		return err
	}

	bin.Config = &schema.BinaryConfig{Command: []string{"/" + bin.Name}}

	pkg.Binaries = append(pkg.Binaries, bin)

	return api.SetServerBinaryRef(pkg, schema.MakePackageRef(pkg.Location.PackageName, bin.Name))
}

func (impl) CreateBinary(ctx context.Context, env *schema.Environment, pl pkggraph.PackageLoader, loc pkggraph.Location, data *schema.GoIntegration) (*schema.Binary, error) {
	goPkg := data.Pkg
	if goPkg == "" {
		goPkg = "."
	}

	rel := loc.Rel(goPkg)

	// TODO consider validating that goPkg is valid (e.g. a Go test and a Go server cannot live in the same package)

	return &schema.Binary{
		Name: filepath.Base(rel),
		BuildPlan: &schema.LayeredImageBuildPlan{
			LayerBuildPlan: []*schema.ImageBuildPlan{{
				GoPackage: goPkg,
			}},
		},
	}, nil
}

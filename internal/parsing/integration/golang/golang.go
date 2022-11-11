// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package integrations

import (
	"context"

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

func (impl) ApplyToServer(ctx context.Context, env *schema.Environment, pl pkggraph.PackageLoader, pkg *pkggraph.Package, data *schema.GoIntegration) error {
	if pkg.Server == nil {
		// Can't happen with the current syntax.
		return fnerrors.NewWithLocation(pkg.Location, "go integration requires a server")
	}

	goPkg := data.Pkg
	if goPkg == "" {
		goPkg = "."
	}

	return api.SetServerBinary(pkg,
		&schema.LayeredImageBuildPlan{
			LayerBuildPlan: []*schema.ImageBuildPlan{{GoPackage: goPkg}},
		},
		[]string{"/" + pkg.Server.Name})
}

func (impl) CreateBinary(ctx context.Context, env *schema.Environment, pl pkggraph.PackageLoader, loc pkggraph.Location, data *schema.GoIntegration) (*schema.Binary, error) {
	goPkg := data.Pkg
	if goPkg == "" {
		goPkg = "."
	}

	// TODO consider validating that goPkg is valid (e.g. a Go test and a Go server cannot live in the same package)

	return &schema.Binary{
		BuildPlan: &schema.LayeredImageBuildPlan{
			LayerBuildPlan: []*schema.ImageBuildPlan{{
				GoPackage: goPkg,
			}},
		},
	}, nil
}

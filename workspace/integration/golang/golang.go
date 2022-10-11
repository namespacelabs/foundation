// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package integrations

import (
	"context"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/workspace/integration/api"
)

func Apply(ctx context.Context, env *schema.Environment, pl pkggraph.PackageLoader, data *schema.GoIntegration, pkg *pkggraph.Package) error {
	if pkg.Server == nil {
		// Can't happen with the current syntax.
		return fnerrors.UserError(pkg.Location, "go integration requires a server")
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

func CreateBinary(ctx context.Context, env *schema.Environment, pl pkggraph.PackageLoader, loc pkggraph.Location, data *schema.GoIntegration) (*schema.Binary, error) {
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

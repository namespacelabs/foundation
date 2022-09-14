// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package integrations

import (
	"context"

	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/internal/integration/api"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type GoIntegration struct {
}

type cueIntegrationGo struct {
	Package string `json:"pkg"`
}

func (i *GoIntegration) Kind() string {
	return "namespace.so/from-go"
}

func (i *GoIntegration) Shortcut() string {
	return "go"
}

func (i *GoIntegration) Parse(ctx context.Context, pkg *pkggraph.Package, v *fncue.CueV) error {
	var bits cueIntegrationGo
	if err := v.Val.Decode(&bits); err != nil {
		return err
	}

	goPkg := bits.Package
	if goPkg == "" {
		goPkg = "."
	}

	return api.SetServerBinary(pkg, &schema.LayeredImageBuildPlan{
		LayerBuildPlan: []*schema.ImageBuildPlan{{GoPackage: goPkg}},
	}, nil)
}

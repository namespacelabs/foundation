// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package integrations

import (
	"context"

	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/workspace/integration/api"
)

type GoIntegrationApplier struct{}

func (i *GoIntegrationApplier) Kind() string { return "namespace.so/from-go" }

func (i *GoIntegrationApplier) Apply(ctx context.Context, dataAny *anypb.Any, pkg *pkggraph.Package) error {
	data := &schema.GoIntegration{}
	if err := dataAny.UnmarshalTo(data); err != nil {
		return err
	}

	if pkg.Server == nil {
		// Can't happen with the current syntax.
		return fnerrors.UserError(pkg.Location, "go integration requires a server")
	}

	goPkg := data.Package
	if goPkg == "" {
		goPkg = "."
	}

	return api.SetServerBinary(pkg,
		&schema.LayeredImageBuildPlan{
			LayerBuildPlan: []*schema.ImageBuildPlan{{GoPackage: goPkg}},
		},
		[]string{"/" + pkg.Server.Name})
}

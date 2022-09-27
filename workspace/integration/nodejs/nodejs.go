// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package nodejs

import (
	"context"

	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/languages/nodejs/binary"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/workspace/integration/api"
)

type NodejsIntegrationApplier struct{}

func (i *NodejsIntegrationApplier) Kind() string { return "namespace.so/from-nodejs" }

func (i *NodejsIntegrationApplier) Apply(ctx context.Context, dataAny *anypb.Any, pkg *pkggraph.Package) error {
	data := &schema.NodejsIntegration{}
	if err := dataAny.UnmarshalTo(data); err != nil {
		return err
	}

	if pkg.Server == nil {
		// Can't happen with the current syntax.
		return fnerrors.UserError(pkg.Location, "nodejs integration requires a server")
	}

	nodePkg := data.Package
	if nodePkg == "" {
		nodePkg = "."
	}

	return api.SetServerBinary(
		pkg,
		&schema.LayeredImageBuildPlan{
			LayerBuildPlan: []*schema.ImageBuildPlan{{
				NodejsBuild: &schema.ImageBuildPlan_NodejsBuild{
					RelPath: nodePkg,
				}}},
		},
		[]string{binary.RunScriptPath})
}

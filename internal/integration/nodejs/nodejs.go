// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package nodejs

import (
	"context"

	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/internal/integration/api"
	"namespacelabs.dev/foundation/languages/nodejs/binary"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type NodejsIntegration struct {
}

type cueIntegrationNodejs struct {
	Package string `json:"pkg"`
}

func (i *NodejsIntegration) Kind() string {
	return "namespace.so/from-nodejs"
}

func (i *NodejsIntegration) Shortcut() string {
	return "nodejs"
}

func (i *NodejsIntegration) Parse(ctx context.Context, pkg *pkggraph.Package, v *fncue.CueV) error {
	var bits cueIntegrationNodejs
	if v != nil {
		if err := v.Val.Decode(&bits); err != nil {
			return err
		}
	}

	nodePkg := bits.Package
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

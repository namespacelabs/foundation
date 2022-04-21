// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package nodejs

import (
	"context"

	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/languages/shared"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
)

const DepsFilename = "deps.fn.ts"

func generateNode(ctx context.Context, loader workspace.Packages, loc workspace.Location, n *schema.Node, nodes []*schema.Node, fs fnfs.ReadWriteFS) error {
	nodeData, err := shared.PrepareNodeData(ctx, loader, loc, n, schema.Framework_NODEJS)
	if err != nil {
		return err
	}

	return generateSource(ctx, fs, loc.Rel(DepsFilename), serviceTmpl, convertNodeDataToTmplOptions(nodeData))
}

func convertNodeDataToTmplOptions(nodeData shared.NodeData) nodeTmplOptions {
	ic := NewImportCollector()

	providers := []tmplProvider{}
	for _, p := range nodeData.Providers {
		providers = append(providers, tmplProvider{
			Name:       p.Name,
			InputType:  convertType(ic, p.InputType),
			OutputType: convertAvailableIn(ic, p.ProviderType.Nodejs),
		})
	}

	var service *tmplService
	if nodeData.Service != nil {
		deps := []tmplDependency{}
		for _, d := range nodeData.Service.Deps {
			deps = append(deps, tmplDependency{
				Name: d.Name,
				Type: convertAvailableIn(ic, d.ProviderType.Nodejs),
			})
		}

		service = &tmplService{
			Deps: deps,
		}
	}

	return nodeTmplOptions{
		Imports:   ic.imports(),
		Service:   service,
		Providers: providers,
	}
}

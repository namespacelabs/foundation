// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package shared

import (
	"context"

	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
)

// Prepare codegen data for a server.
func PrepareServerData(ctx context.Context, loader workspace.Packages, loc workspace.Location, srv *schema.Server) (ServerData, error) {
	var serverData ServerData

	for _, ref := range srv.GetImportedPackages() {
		pkg, err := loader.LoadByName(ctx, ref)
		if err != nil {
			return serverData, err
		}

		if pkg.Node().GetKind() == schema.Node_SERVICE {
			serverData.Services = append(serverData.Services, EmbeddedServiceData{
				Location: pkg.Location,
			})
		}
	}

	return serverData, nil
}

func PrepareNodeData(ctx context.Context, loader workspace.Packages, loc workspace.Location, n *schema.Node, fmwk schema.Framework) (NodeData, error) {
	var nodeData NodeData

	if n.ExportService != nil {
		nodeData.Service = &ServiceData{}
	}
	for _, p := range n.Provides {
		for _, a := range p.AvailableIn {
			if a.ProvidedInFrameworks()[fmwk] {
				nodeData.Providers = append(nodeData.Providers, ProviderData{
					Name:         p.Name,
					InputType:    p.Type,
					ProviderType: a,
				})
			}
		}
	}

	return nodeData, nil
}

// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package shared

import (
	"context"

	"namespacelabs.dev/foundation/internal/fnerrors"
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
		deps := []DependencyData{}
		for _, dep := range n.GetInstantiate() {
			pkg, err := loader.LoadByName(ctx, schema.PackageName(dep.PackageName))
			if err != nil {
				return NodeData{}, fnerrors.UserError(nil, "failed to load %s/%s: %w", dep.PackageName, dep.Type, err)
			}

			_, p := workspace.FindProvider(pkg, schema.PackageName(dep.PackageName), dep.Type)
			if p == nil {
				return NodeData{}, fnerrors.UserError(nil, "didn't find a provider for %s/%s", dep.PackageName, dep.Type)
			}

			var provider *schema.Provides_AvailableIn
			for _, prov := range p.AvailableIn {
				if prov.ProvidedInFrameworks()[fmwk] {
					provider = prov
					break
				}
			}

			deps = append(deps, DependencyData{
				Name:         dep.Name,
				ProviderType: provider,
			})
		}

		nodeData.Service = &ServiceData{
			Deps: deps,
		}
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

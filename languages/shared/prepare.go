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
func PrepareServerData(ctx context.Context, loader workspace.Packages, loc workspace.Location, srv *schema.Server, nodes []*schema.Node) (ServerData, error) {
	services := []EmbeddedServiceData{}
	packageToNode := map[string]*schema.Node{}
	for _, n := range nodes {
		packageToNode[n.PackageName] = n
	}
	for _, ref := range srv.GetImportedPackages() {
		referencedNode := packageToNode[ref.String()]
		if referencedNode == nil {
			return ServerData{}, fnerrors.InternalError("%s: package not loaded?", ref)
		}

		refLoc, err := loader.Resolve(ctx, ref)
		if err != nil {
			return ServerData{}, err
		}

		if referencedNode.GetKind() == schema.Node_SERVICE {
			services = append(services, EmbeddedServiceData{
				Location: refLoc,
			})
		}
	}

	return ServerData{
		Services: services,
	}, nil
}

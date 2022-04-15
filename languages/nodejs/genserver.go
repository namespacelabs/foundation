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

const ServerFilename = "main.fn.ts"

func generateServer(ctx context.Context, loader workspace.Packages, loc workspace.Location, srv *schema.Server, nodes []*schema.Node, fs fnfs.ReadWriteFS) error {
	serverData, err := shared.PrepareServerData(ctx, loader, loc, srv)
	if err != nil {
		return err
	}

	tplImports := []singleImport{}
	tplServices := []service{}
	for _, srv := range serverData.Services {
		nodejsLoc, err := nodejsLocationFrom(srv.Location.PackageName)
		if err != nil {
			return err
		}

		tplServices = append(tplServices, service{
			Name:        nodejsLoc.Name,
			ImportAlias: nodejsLoc.Name,
		})
		tplImports = append(tplImports, singleImport{
			Alias:   nodejsLoc.Name,
			Package: nodejsServiceDepsImport(nodejsLoc.NpmPackage),
		})
	}

	return generateSource(ctx, fs, loc.Rel(ServerFilename), serverTmpl, serverTmplOptions{
		Imports:  tplImports,
		Services: tplServices,
	})
}

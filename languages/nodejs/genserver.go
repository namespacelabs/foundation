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

	ic := NewImportCollector()
	tplServices := []tmplImportedType{}
	for _, srv := range serverData.Services {
		nodejsLoc, err := nodejsLocationFrom(srv.Location.PackageName)
		if err != nil {
			return err
		}

		alias := ic.add(nodejsServiceDepsImport(nodejsLoc.NpmPackage))
		tplServices = append(tplServices, tmplImportedType{
			Name:        nodejsLoc.Name,
			ImportAlias: alias,
		})
	}

	return generateSource(ctx, fs, loc.Rel(ServerFilename), serverTmpl, serverTmplOptions{
		Imports:  ic.imports(),
		Services: tplServices,
	})
}

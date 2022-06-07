// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package integration

import (
	"context"
	"strings"

	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/languages/nodejs/imports"
	"namespacelabs.dev/foundation/languages/shared"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
)

const ServerFilename = "main.fn.ts"

func generateServer(ctx context.Context, loader workspace.Packages, loc workspace.Location, srv *schema.Server, nodes []*schema.Node, fs fnfs.ReadWriteFS) error {
	serverData, err := shared.PrepareServerData(ctx, loader, loc, srv, schema.Framework_NODEJS)
	if err != nil {
		return err
	}

	ic := imports.NewImportCollector()
	tplServices := []tmplServerService{}
	for _, srv := range serverData.Services {
		npmPackage, err := toNpmPackage(srv.Location)
		if err != nil {
			return err
		}

		pkgComponents := strings.Split(string(srv.Location.PackageName), "/")
		srvName := pkgComponents[len(pkgComponents)-1]

		tplServices = append(tplServices, tmplServerService{
			HasDeps: srv.HasDeps,
			Type: tmplImportedType{
				Name:        srvName,
				ImportAlias: ic.Add(nodeDepsNpmImport(npmPackage)),
			}})
	}

	importedInitializersAliases, err := convertImportedInitializers(ic, serverData.ImportedInitializers)
	if err != nil {
		return err
	}

	return generateSource(ctx, fs, loc.Rel(ServerFilename), tmpl, "Server", serverTmplOptions{
		Imports:                     ic.Imports(),
		Services:                    tplServices,
		ImportedInitializersAliases: importedInitializersAliases,
	})
}

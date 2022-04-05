// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package nodejs

import (
	"context"
	"io"
	"text/template"

	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
)

const ServiceDepsFilename = "deps.fn.ts"

func generateNode(ctx context.Context, loader workspace.Packages, loc workspace.Location, n *schema.Node, nodes []*schema.Node, fs fnfs.ReadWriteFS) error {
	if len(n.ExportService) == 0 {
		return nil
	}

	return generateSource(ctx, fs, loc.Rel(ServiceDepsFilename), serviceTmpl, nodeTmplOptions{
		Imports:       []singleImport{},
		NeedsDepsType: true,
		DepVars: []depVar{{
			Name: "myDep",
			Type: typeDef{
				ImportAlias: "",
				Name:        "string",
			},
		}},
	})
}

func generateSource(ctx context.Context, fsfs fnfs.ReadWriteFS, filePath string, t *template.Template, data interface{}) error {
	return fnfs.WriteWorkspaceFile(ctx, fsfs, filePath, func(w io.Writer) error {
		return WriteSource(w, t, data)
	})
}

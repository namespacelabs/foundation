// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package nodejs

import (
	"context"

	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
)

const DepsFilename = "deps.fn.ts"

func generateNode(ctx context.Context, loader workspace.Packages, loc workspace.Location, n *schema.Node, nodes []*schema.Node, fs fnfs.ReadWriteFS) error {
	// Only services for now.
	if len(n.ExportService) == 0 {
		return nil
	}

	return generateSource(ctx, fs, loc.Rel(DepsFilename), serviceTmpl, nodeTmplOptions{
		NeedsDepsType: len(n.ExportService) != 0,
	})
}

// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package golang

import (
	"context"
	"io"
	"text/template"

	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/gosupport"
)

func generateGoSource(ctx context.Context, fsfs fnfs.ReadWriteFS, filePath string, t *template.Template, data interface{}) error {
	return fnfs.WriteWorkspaceFile(ctx, fsfs, filePath, func(w io.Writer) error {
		return gosupport.WriteGoSource(w, t, data)
	})
}
// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package golang

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"text/template"

	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/gosupport"
)

func generateGoSource(ctx context.Context, fsfs fnfs.ReadWriteFS, filePath string, t *template.Template, data interface{}) error {
	return fnfs.WriteWorkspaceFile(ctx, fsfs, filePath, func(w io.Writer) error {
		if err := gosupport.WriteGoSource(w, t, data); err != nil {
			var b bytes.Buffer
			_ = t.Execute(&b, data)
			fmt.Fprintln(console.Debug(ctx), b.String())
			return err
		}
		return nil
	})
}

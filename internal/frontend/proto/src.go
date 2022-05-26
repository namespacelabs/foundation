// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package proto

import (
	"bytes"
	"context"
	"io"
	"text/template"

	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
)

func createProtoScaffold(ctx context.Context, fsfs fnfs.ReadWriteFS, filePath string, t *template.Template, data interface{}) error {
	return fnfs.WriteWorkspaceFile(ctx, console.Stdout(ctx), fsfs, filePath, func(w io.Writer) error {
		var body bytes.Buffer

		if err := t.Execute(&body, data); err != nil {
			return fnerrors.InternalError("failed to apply template: %w", err)
		}

		var src bytes.Buffer

		if _, err := body.WriteTo(&src); err != nil {
			return err
		}

		// TODO run proto formatter

		_, err := w.Write(src.Bytes())
		return err
	})
}

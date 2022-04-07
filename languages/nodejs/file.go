// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package nodejs

import (
	"bytes"
	"context"
	"io"
	"text/template"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
)

func generateSource(ctx context.Context, fsfs fnfs.ReadWriteFS, filePath string, t *template.Template, data interface{}) error {
	return fnfs.WriteWorkspaceFile(ctx, fsfs, filePath, func(w io.Writer) error {
		// TODO(@nicolasalt): format the file.
		return writeSource(w, t, data)
	})
}

func writeSource(w io.Writer, t *template.Template, data interface{}) error {
	var b bytes.Buffer

	if err := t.Execute(&b, data); err != nil {
		return fnerrors.InternalError("failed to apply template: %w", err)
	}

	// TODO: format the generated Typescript code.

	_, err := w.Write(b.Bytes())
	return err
}

// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cue

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"text/template"

	"cuelang.org/go/cue/format"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
)

func generateCueSource(ctx context.Context, fsfs fnfs.ReadWriteFS, filePath string, t *template.Template, data interface{}) error {
	return fnfs.WriteWorkspaceFile(ctx, console.Stdout(ctx), fsfs, filePath, func(w io.Writer) error {
		var body bytes.Buffer

		if err := t.Execute(&body, data); err != nil {
			return fnerrors.InternalError("failed to apply template: %w", err)
		}

		var src bytes.Buffer

		if _, err := body.WriteTo(&src); err != nil {
			return err
		}

		formatted, err := format.Source(src.Bytes())
		if err != nil {
			fmt.Fprintln(console.Debug(ctx), "The input sources were:")
			fmt.Fprintln(console.Debug(ctx), src.String())

			return fnerrors.InternalError("failed to format generated Cue file: %w", err)
		}

		_, err = w.Write(formatted)
		return err
	})
}

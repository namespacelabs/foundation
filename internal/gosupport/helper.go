// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package gosupport

import (
	"bytes"
	"go/format"
	"io"
	"text/template"

	"namespacelabs.dev/foundation/internal/fnerrors"
)

func WriteGoSource(w io.Writer, t *template.Template, data interface{}) error {
	var b bytes.Buffer

	if err := t.Execute(&b, data); err != nil {
		return fnerrors.InternalError("failed to apply template: %w", err)
	}

	formatted, err := format.Source(b.Bytes())
	if err != nil {
		return fnerrors.InternalError("failed to format generated Go file: %w", err)
	}

	_, err = io.Copy(w, bytes.NewReader(formatted))
	return err
}
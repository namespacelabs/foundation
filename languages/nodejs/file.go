// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package nodejs

import (
	"bytes"
	"io"
	"text/template"

	"namespacelabs.dev/foundation/internal/fnerrors"
)

func WriteSource(w io.Writer, t *template.Template, data interface{}) error {
	var b bytes.Buffer

	if err := t.Execute(&b, data); err != nil {
		return fnerrors.InternalError("failed to apply template: %w", err)
	}

	// TODO: format

	_, err := w.Write(b.Bytes())
	return err
}

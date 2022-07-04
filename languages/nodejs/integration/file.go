// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package integration

import (
	"bytes"
	"context"
	"io"
	"path/filepath"
	"text/template"

	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/findroot"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/workspace"
)

func generateSource(ctx context.Context, fsfs fnfs.ReadWriteFS, filePath string, t *template.Template, templateName string, data interface{}) error {
	return fnfs.WriteWorkspaceFile(ctx, console.Stdout(ctx), fsfs, filePath, func(w io.Writer) error {
		// TODO(@nicolasalt): format the file.
		return writeSource(w, t, templateName, data)
	})
}

func writeSource(w io.Writer, t *template.Template, templateName string, data interface{}) error {
	var b bytes.Buffer

	if err := t.ExecuteTemplate(&b, templateName, data); err != nil {
		return fnerrors.InternalError("failed to apply template: %w", err)
	}

	// TODO: format the generated Typescript code.

	_, err := w.Write(b.Bytes())
	return err
}

func findYarnRoot(loc workspace.Location) (string, error) {
	path, err := findroot.Find(packageJsonFn, loc.Abs(), findroot.LookForFile(packageJsonFn))
	if err != nil {
		return "", nil
	}

	relPath, err := filepath.Rel(loc.Module.Abs(), path)
	if err != nil {
		return "", err
	}

	return relPath, nil
}

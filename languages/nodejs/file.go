// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package nodejs

import (
	"bytes"
	"context"
	"io"
	"path/filepath"
	"text/template"

	"namespacelabs.dev/foundation/internal/findroot"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
)

const yarnLockFn = "yarn.lock"

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

func findYarnRoot(loc workspace.Location) (schema.PackageName, error) {
	path, err := findroot.Find(loc.Abs(), findroot.LookForFile(yarnLockFn))
	if err != nil {
		return "", fnerrors.UserError(nil, "Couldn't find %s: %w", yarnLockFn, err)
	}

	relPath, err := filepath.Rel(loc.Module.Abs(), path)
	if err != nil {
		return "", err
	}

	return schema.Name(relPath), nil
}

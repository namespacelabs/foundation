// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package nodejs

import (
	"context"
	"fmt"
	"io"
	"strings"
	"text/template"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/schema"
)

func nodejsLocationFrom(pkgName schema.PackageName) (nodejsLocation, error) {
	pkgComponents := strings.Split(string(pkgName), "/")
	if len(pkgComponents) < 2 {
		return nodejsLocation{}, fnerrors.InternalError("Invalid package name: %s", pkgName)
	}
	npmName := strings.Join(pkgComponents[1:], "_")
	return nodejsLocation{
		Name:       pkgComponents[len(pkgComponents)-1],
		NpmPackage: fmt.Sprintf("@%s/%s", pkgComponents[0], npmName),
	}, nil
}

func nodejsServiceDepsImport(npmPackage string) string {
	return fmt.Sprintf("%s/deps.fn", npmPackage)
}

type nodejsLocation struct {
	Name       string
	NpmPackage string
}

func generateSource(ctx context.Context, fsfs fnfs.ReadWriteFS, filePath string, t *template.Template, data interface{}) error {
	return fnfs.WriteWorkspaceFile(ctx, fsfs, filePath, func(w io.Writer) error {
		// TODO(@nicolasalt): format the file.
		return WriteSource(w, t, data)
	})
}

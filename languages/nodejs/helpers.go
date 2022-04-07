// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package nodejs

import (
	"fmt"
	"strings"

	"namespacelabs.dev/foundation/internal/fnerrors"
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

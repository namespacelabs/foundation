// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package nodejs

import (
	"fmt"
	"strings"

	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/languages/shared"
	"namespacelabs.dev/foundation/schema"
)

type NpmPackage string

func toNpmPackage(pkgName schema.PackageName) (NpmPackage, error) {
	pkgComponents := strings.Split(string(pkgName), "/")
	if len(pkgComponents) < 2 {
		return "", fnerrors.InternalError("Invalid package name: %s", pkgName)
	}
	npmName := strings.Join(pkgComponents[1:], "_")
	return NpmPackage(fmt.Sprintf("@%s/%s", pkgComponents[0], npmName)), nil
}

func npmImport(npmPackage NpmPackage, moduleName string) string {
	return fmt.Sprintf("%s/%s", npmPackage, moduleName)
}

func nodeDepsNpmImport(npmPackage NpmPackage) string {
	return npmImport(npmPackage, "deps.fn")
}

func convertPackageToImport(pkg schema.PackageName, filename string) (string, error) {
	moduleName := filename

	if strings.HasSuffix(filename, ".proto") {
		// strip suffix
		moduleName = strings.TrimSuffix(moduleName, ".proto") + "_pb"
	}

	npmPackage, err := toNpmPackage(pkg)
	if err != nil {
		return "", err
	}

	return npmImport(npmPackage, moduleName), nil
}

func convertType(ic *importCollector, t shared.TypeData) (tmplImportedType, error) {
	// TODO(@nicolasalt): handle the case when the source type is not in the same package.
	npmImport, err := convertPackageToImport(t.PackageName, t.SourceFileName)
	if err != nil {
		return tmplImportedType{}, err
	}

	return tmplImportedType{
		Name:        t.Name,
		ImportAlias: ic.add(npmImport),
	}, nil
}

func convertAvailableIn(ic *importCollector, a *schema.Provides_AvailableIn_NodeJs) tmplImportedType {
	return tmplImportedType{
		Name:        a.Type,
		ImportAlias: ic.add(a.Import),
	}
}

type importCollector struct {
	// Key: pkg
	pkgToImport map[string]tmplSingleImport
	aliasIndex  int
}

func newImportCollector() *importCollector {
	return &importCollector{
		pkgToImport: make(map[string]tmplSingleImport),
	}
}

// Returns assigned alias
func (ic *importCollector) add(npmImport string) string {
	var alias string
	if im, ok := ic.pkgToImport[npmImport]; ok {
		alias = im.Alias
	} else {
		alias = fmt.Sprintf("i%d", ic.aliasIndex)
		ic.aliasIndex++
		ic.pkgToImport[npmImport] = tmplSingleImport{
			Alias:   alias,
			Package: npmImport,
		}
	}

	return alias
}

func (ic *importCollector) imports() []tmplSingleImport {
	imports := make([]tmplSingleImport, 0, len(ic.pkgToImport))
	for _, imp := range ic.pkgToImport {
		imports = append(imports, imp)
	}
	slices.SortFunc(imports, func(i1, i2 tmplSingleImport) bool {
		return i1.Alias < i2.Alias
	})

	return imports
}

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

func convertPackageToImport(pkg string) string {
	if strings.HasSuffix(pkg, ".proto") {
		// strip suffix
		pkg = strings.TrimSuffix(pkg, ".proto") + "_pb"
	}

	// Local paths
	if !strings.Contains(pkg, "/") {
		pkg = "./" + pkg
	}

	return pkg
}

func convertType(ic *importCollector, t *schema.TypeDef) importedType {
	nameParts := strings.Split(t.Typename, ".")
	return importedType{
		Name:        nameParts[len(nameParts)-1],
		ImportAlias: ic.add(convertPackageToImport(t.Source[0])),
	}
}

func convertAvailableIn(ic *importCollector, a *schema.Provides_AvailableIn_NodeJs) importedType {
	return importedType{
		Name:        a.Type,
		ImportAlias: ic.add(a.Import),
	}
}

type importCollector struct {
	// Key: pkg
	pkgToImport map[string]singleImport
	aliasIndex  int
}

func NewImportCollector() *importCollector {
	return &importCollector{
		pkgToImport: make(map[string]singleImport),
	}
}

// Returns assigned alias
func (ic *importCollector) add(pkg string) string {
	var alias string
	if im, ok := ic.pkgToImport[pkg]; ok {
		alias = im.Alias
	} else {
		alias = fmt.Sprintf("i%d", ic.aliasIndex)
		ic.aliasIndex++
		ic.pkgToImport[pkg] = singleImport{
			Alias:   alias,
			Package: pkg,
		}
	}

	return alias
}

func (ic *importCollector) imports() []singleImport {
	imports := make([]singleImport, 0, len(ic.pkgToImport))
	for _, imp := range ic.pkgToImport {
		imports = append(imports, imp)
	}

	return imports
}

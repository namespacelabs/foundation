// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package integration

import (
	"fmt"
	"strings"

	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/languages/shared"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
)

type NpmPackage string

func toNpmPackage(loc workspace.Location) (NpmPackage, error) {
	pkg := strings.Join(strings.Split(loc.Rel(), "/"), "-")
	namespace := strings.Join(strings.Split(loc.Module.ModuleName(), "/"), "-")
	return NpmPackage(fmt.Sprintf("@%s/%s", namespace, pkg)), nil
}

func npmImport(npmPackage NpmPackage, moduleName string) string {
	return fmt.Sprintf("%s/%s", npmPackage, moduleName)
}

func nodeDepsNpmImport(npmPackage NpmPackage) string {
	return npmImport(npmPackage, "deps.fn")
}

func convertLocationToImport(loc workspace.Location, filename string) (string, error) {
	moduleName := filename

	if strings.HasSuffix(filename, ".proto") {
		// strip suffix
		moduleName = strings.TrimSuffix(moduleName, ".proto") + "_pb"
	}

	npmPackage, err := toNpmPackage(loc)
	if err != nil {
		return "", err
	}

	return npmImport(npmPackage, moduleName), nil
}

func convertType(ic *importCollector, t shared.TypeData) (tmplImportedType, error) {
	// TODO(@nicolasalt): handle the case when the source type is not in the same package.
	npmImport, err := convertLocationToImport(t.Location, t.SourceFileName)
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

func convertImportedInitializers(ic *importCollector, locations []workspace.Location) ([]string, error) {
	result := []string{}
	for _, loc := range locations {
		npmPackage, err := toNpmPackage(loc)
		if err != nil {
			return nil, err
		}
		result = append(result, ic.add(nodeDepsNpmImport(npmPackage)))
	}

	return result, nil
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

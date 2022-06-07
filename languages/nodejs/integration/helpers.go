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

var (
	npmNamespaceAliases = map[schema.PackageName]NpmPackage{
		runtimeNode: runtimeNpmPackage,
	}
)

type NpmPackage string

func toNpmNamespace(moduleName string) string {
	return strings.Join(strings.Split(moduleName, "/"), "-")
}

func toNpmPackage(loc workspace.Location) (NpmPackage, error) {
	if pkg, ok := npmNamespaceAliases[loc.PackageName]; ok {
		return NpmPackage(pkg), nil
	}

	pkg := strings.Join(strings.Split(loc.Rel(), "/"), "-")
	return NpmPackage(fmt.Sprintf("@%s/%s", toNpmNamespace(loc.Module.ModuleName()), pkg)), nil
}

func npmImport(npmPackage NpmPackage, moduleName string) string {
	return fmt.Sprintf("%s/%s", npmPackage, moduleName)
}

func nodeDepsNpmImport(npmPackage NpmPackage) string {
	return npmImport(npmPackage, "deps.fn")
}

func convertProtoType(ic *importCollector, t shared.ProtoTypeData) (tmplImportedType, error) {
	tsModuleName := t.SourceFileName

	if strings.HasSuffix(tsModuleName, ".proto") {
		// strip suffix
		tsModuleName = strings.TrimSuffix(tsModuleName, ".proto")
		if t.Kind == shared.ProtoService {
			tsModuleName += "_grpc_pb"
		} else {
			tsModuleName += "_pb"
		}
	}

	npmPackage, err := toNpmPackage(t.Location)
	if err != nil {
		return tmplImportedType{}, err
	}

	var typeName string
	if t.Kind == shared.ProtoService {
		typeName = fmt.Sprintf("%sClient", t.Name)
	} else {
		typeName = t.Name
	}

	return tmplImportedType{
		Name: typeName,
		// TODO: handle the case when the source type is not in the same package.
		ImportAlias: ic.add(npmImport(npmPackage, tsModuleName)),
	}, nil
}

func convertAvailableIn(ic *importCollector, a *schema.Provides_AvailableIn_NodeJs, loc workspace.Location) (tmplImportedType, error) {
	// Empty import means that the type is generated at runtime when the provider is used as a dependency,
	// and here were are generating the provider definition. In this case this type is not used from the templates.
	if a.Import == "" {
		return tmplImportedType{}, nil
	}

	var imp string
	if strings.Contains(a.Import, "/") {
		// Full path is specified.
		imp = a.Import
	} else {
		// As a shortcut, the user can specify the file from the same package without the full NPM package.
		npmPackage, err := toNpmPackage(loc)
		if err != nil {
			return tmplImportedType{}, err
		}
		imp = npmImport(npmPackage, a.Import)
	}

	if strings.HasPrefix(a.Type, "Promise<") {
		innerType, err := convertAvailableIn(ic, &schema.Provides_AvailableIn_NodeJs{
			Import: a.Import,
			Type:   strings.TrimSuffix(strings.TrimPrefix(a.Type, "Promise<"), ">"),
		}, loc)
		if err != nil {
			return tmplImportedType{}, err
		}

		return tmplImportedType{
			Name:       "Promise",
			Parameters: []tmplImportedType{innerType},
		}, nil
	} else {
		return tmplImportedType{
			Name:        a.Type,
			ImportAlias: ic.add(imp),
		}, nil
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

// Returns the assigned alias or an empty string if no alias has been assigned.
func (ic *importCollector) add(npmImport string) string {
	if npmImport == "" {
		return ""
	}

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

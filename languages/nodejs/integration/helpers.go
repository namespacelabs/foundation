// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package integration

import (
	"fmt"
	"strings"

	"namespacelabs.dev/foundation/languages/nodejs/imports"
	"namespacelabs.dev/foundation/languages/shared"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type NpmPackage string

func toNpmNamespace(moduleName string) string {
	parts := strings.Split(moduleName, "/")
	if len(parts) > 1 {
		return fmt.Sprintf("@%s/%s", strings.Join(parts[:len(parts)-1], "-"), parts[len(parts)-1])
	} else {
		return moduleName
	}
}

func toNpmPackage(loc pkggraph.Location) (NpmPackage, error) {
	return NpmPackage(fmt.Sprintf("%s/%s", toNpmNamespace(loc.Module.ModuleName()), loc.Rel())), nil
}

func npmImport(npmPackage NpmPackage, moduleName string) string {
	return fmt.Sprintf("%s/%s", npmPackage, moduleName)
}

func nodeDepsNpmImport(npmPackage NpmPackage) string {
	return npmImport(npmPackage, "deps.fn")
}

func convertProtoType(ic *imports.ImportCollector, t shared.ProtoTypeData) (*tmplImportedType, error) {
	tsModuleName := t.SourceFileName

	if strings.HasSuffix(tsModuleName, ".proto") {
		// strip suffix
		tsModuleName = strings.TrimSuffix(tsModuleName, ".proto")
		if t.Kind == shared.ProtoService {
			tsModuleName = strings.TrimSuffix(generatedGrpcFilePath(tsModuleName), ".ts")
		} else {
			tsModuleName += "_pb"
		}
	}

	// Hack: protobuf-ts doesn't support "import public" in proto files so manually substitute
	// the import for the standard gRPC extension.
	if string(t.Location.PackageName) == "namespacelabs.dev/foundation/std/grpc" && t.SourceFileName == "provider.proto" {
		tsModuleName = "protos/" + tsModuleName
	}

	npmPackage, err := toNpmPackage(t.Location)
	if err != nil {
		return nil, err
	}

	var typeName string
	if t.Kind == shared.ProtoService {
		typeName = fmt.Sprintf("%sClient", t.Name)
	} else {
		typeName = t.Name
	}

	return &tmplImportedType{
		Name: typeName,
		// TODO: handle the case when the source type is not in the same package.
		ImportAlias: ic.Add(npmImport(npmPackage, tsModuleName)),
	}, nil
}

func convertAvailableIn(ic *imports.ImportCollector, a *schema.Provides_AvailableIn_NodeJs, loc pkggraph.Location) (*tmplImportedType, error) {
	// Empty import means that the type is generated at runtime when the provider is used as a dependency,
	// and here were are generating the provider definition. In this case this type is not used from the templates.
	if a.Import == "" {
		return &tmplImportedType{}, nil
	}

	var imp string
	if strings.Contains(a.Import, "/") {
		// Full path is specified.
		imp = a.Import
	} else {
		// As a shortcut, the user can specify the file from the same package without the full NPM package.
		npmPackage, err := toNpmPackage(loc)
		if err != nil {
			return nil, err
		}
		imp = npmImport(npmPackage, a.Import)
	}

	if strings.HasPrefix(a.Type, "Promise<") {
		innerType, err := convertAvailableIn(ic, &schema.Provides_AvailableIn_NodeJs{
			Import: a.Import,
			Type:   strings.TrimSuffix(strings.TrimPrefix(a.Type, "Promise<"), ">"),
		}, loc)
		if err != nil {
			return nil, err
		}

		return &tmplImportedType{
			Name:       "Promise",
			Parameters: []tmplImportedType{*innerType},
		}, nil
	} else {
		return &tmplImportedType{
			Name:        a.Type,
			ImportAlias: ic.Add(imp),
		}, nil
	}
}

func convertImportedInitializers(ic *imports.ImportCollector, locations []pkggraph.Location) ([]string, error) {
	result := []string{}
	for _, loc := range locations {
		npmPackage, err := toNpmPackage(loc)
		if err != nil {
			return nil, err
		}
		result = append(result, ic.Add(nodeDepsNpmImport(npmPackage)))
	}

	return result, nil
}

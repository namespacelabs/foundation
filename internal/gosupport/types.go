// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package gosupport

import (
	"fmt"
	"strings"

	"github.com/iancoleman/strcase"
)

type TypeDef struct {
	GoImportURL, GoTypeName, GoName string
}

func MakeType(imports *GoImports, pkg, typeName string) string {
	if pkg == "" {
		return typeName
	}

	if pkg == imports.PkgName {
		return typeName
	}

	// If there's a type qualifier (pointer, slice) present, split it from the type name,
	// so we can move it to be before the package name below.
	qual := ""
	for _, q := range []string{"[]", "*"} {
		if t := strings.TrimPrefix(typeName, q); t != typeName {
			qual += q
			typeName = t
		}
	}

	return fmt.Sprintf("%s%s.%s", qual, imports.MustGet(pkg), typeName)
}

func (dp TypeDef) MakeType(imports *GoImports) string {
	return MakeType(imports, dp.GoImportURL, dp.GoTypeName)
}

func MakeGoPrivVar(input string) string {
	return strcase.ToLowerCamel(input)
}

func MakeGoPubVar(input string) string {
	return strcase.ToCamel(input)
}
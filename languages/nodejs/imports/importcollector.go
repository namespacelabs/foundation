// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package imports

import (
	"fmt"

	"golang.org/x/exp/slices"
)

type ImportCollector struct {
	// Key: pkg
	pkgToImport map[string]SingleImport
	aliasIndex  int
}

type SingleImport struct {
	Alias, Package string
}

func NewImportCollector() *ImportCollector {
	return &ImportCollector{
		pkgToImport: make(map[string]SingleImport),
	}
}

// Returns the assigned alias or an empty string if no alias has been assigned.
func (ic *ImportCollector) Add(npmImport string) string {
	if npmImport == "" {
		return ""
	}

	var alias string
	if im, ok := ic.pkgToImport[npmImport]; ok {
		alias = im.Alias
	} else {
		alias = fmt.Sprintf("i%d", ic.aliasIndex)
		ic.aliasIndex++
		ic.pkgToImport[npmImport] = SingleImport{
			Alias:   alias,
			Package: npmImport,
		}
	}

	return alias
}

func (ic *ImportCollector) Imports() []SingleImport {
	imports := make([]SingleImport, 0, len(ic.pkgToImport))
	for _, imp := range ic.pkgToImport {
		imports = append(imports, imp)
	}
	slices.SortFunc(imports, func(i1, i2 SingleImport) bool {
		return i1.Alias < i2.Alias
	})

	return imports
}

// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package gosupport

import (
	"fmt"
	"regexp"
	"strings"
)

type GoImports struct {
	PkgName string

	urls   []string
	urlmap map[string]string
	revmap map[string]string
}

func NewGoImports(pkgName string) *GoImports {
	return &GoImports{
		PkgName: pkgName,
		urlmap:  map[string]string{},
		revmap:  map[string]string{"init": "forbiddenImport"},
	}
}

type singleImport struct {
	Rename, TypeURL string
}

func (gi *GoImports) Has(typeUrl string) bool {
	for _, u := range gi.urls {
		if u == typeUrl {
			return true
		}
	}
	return false
}

func (gi *GoImports) ImportMap() []singleImport {
	var imports []singleImport
	for _, u := range gi.urls {
		imp := singleImport{TypeURL: u}
		rename := heuristicPackageName(u)
		if rename != gi.urlmap[u] {
			imp.Rename = gi.urlmap[u]
		}
		imports = append(imports, imp)
	}
	return imports
}

var reMatchVer = regexp.MustCompile("^v[0-9]+$")

func heuristicPackageName(p string) string {
	parts := strings.Split(p, "/")

	// If the last url segment is a "version" segment, skip it for
	// name generation purposes.
	if reMatchVer.MatchString(parts[len(parts)-1]) {
		parts = parts[:len(parts)-1]
	}

	return parts[len(parts)-1]
}

func (gi *GoImports) AddOrGet(typeUrl string) {
	if typeUrl == gi.PkgName {
		return
	}

	if _, ok := gi.urlmap[typeUrl]; ok {
		return
	}

	base := heuristicPackageName(typeUrl)

	var rename string
	if _, ok := gi.revmap[base]; !ok {
		gi.revmap[base] = typeUrl
		rename = base
	}

	if rename == "" && strings.HasPrefix(typeUrl, "namespacelabs.dev/foundation/") {
		base = "fn" + base

		if _, ok := gi.revmap[base]; !ok {
			gi.revmap[base] = typeUrl
			rename = base
		}
	}

	if rename == "" {
		for k := 1; ; k++ {
			rename = fmt.Sprintf("%s%d", base, k)
			if _, ok := gi.revmap[rename]; !ok {
				gi.revmap[rename] = typeUrl
				break
			}
		}
	}

	gi.urlmap[typeUrl] = rename
	gi.urls = append(gi.urls, typeUrl) // Retain order.
}

func (gi *GoImports) MustGet(typeUrl string) string {
	rel, ok := gi.urlmap[typeUrl]
	if ok {
		return rel
	}

	panic(typeUrl + " is not known")
}

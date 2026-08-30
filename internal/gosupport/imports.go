// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package gosupport

import (
	"fmt"
	"path"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

type GoImports struct {
	PkgName string

	urls     []string
	urlmap   map[string]string
	revmap   map[string]string
	reserved map[string]struct{}
}

func NewGoImports(pkgName string) *GoImports {
	return &GoImports{
		PkgName:  pkgName,
		urlmap:   map[string]string{},
		revmap:   map[string]string{},
		reserved: map[string]struct{}{"init": {}},
	}
}

type singleImport struct {
	Rename, TypeURL string
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

// notIdentifier reports whether ch is an invalid identifier character.
func notIdentifier(ch rune) bool {
	return !('a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z' ||
		'0' <= ch && ch <= '9' ||
		ch == '_' ||
		ch >= utf8.RuneSelf && (unicode.IsLetter(ch) || unicode.IsDigit(ch)))
}

// Copy of ImportPathToAssumedName
// https://github.com/golang/tools/blob/ff00c7bd7281c81e2b7fcf28446044157439f902/internal/imports/fix.go#L1252C6-L1252C29
func heuristicPackageName(importPath string) string {
	base := path.Base(importPath)
	if strings.HasPrefix(base, "v") {
		if _, err := strconv.Atoi(base[1:]); err == nil {
			dir := path.Dir(importPath)
			if dir != "." {
				base = path.Base(dir)
			}
		}
	}
	base = strings.TrimPrefix(base, "go-")
	if i := strings.IndexFunc(base, notIdentifier); i >= 0 {
		base = base[:i]
	}
	return base
}

func (gi *GoImports) isValidAndNew(name string) bool {
	if _, reserved := gi.reserved[name]; reserved {
		return false
	}

	_, ok := gi.revmap[name]
	return !ok
}

func (gi *GoImports) Ensure(typeUrl string) string {
	if typeUrl == gi.PkgName {
		return ""
	}

	if rename, ok := gi.urlmap[typeUrl]; ok {
		return rename + "."
	}

	base := heuristicPackageName(typeUrl)

	var rename string
	if gi.isValidAndNew(base) {
		gi.revmap[base] = typeUrl
		rename = base
	}

	if rename == "" && strings.HasPrefix(typeUrl, "namespacelabs.dev/foundation/std/") {
		base = "fn" + base

		if gi.isValidAndNew(base) {
			gi.revmap[base] = typeUrl
			rename = base
		}
	}

	if rename == "" {
		for k := 1; ; k++ {
			rename = fmt.Sprintf("%s%d", base, k)
			if gi.isValidAndNew(rename) {
				gi.revmap[rename] = typeUrl
				break
			}
		}
	}

	gi.urlmap[typeUrl] = rename
	gi.urls = append(gi.urls, typeUrl) // Retain order.
	return rename + "."
}

func (gi *GoImports) MustGet2(typeUrl string) string {
	if typeUrl == gi.PkgName {
		return ""
	}

	rel, ok := gi.urlmap[typeUrl]
	if ok {
		return rel + "."
	}

	panic(typeUrl + " is not known")
}

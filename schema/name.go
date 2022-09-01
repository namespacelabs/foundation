// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package schema

import (
	"path/filepath"
	"strings"

	"namespacelabs.dev/foundation/internal/uniquestrings"
)

type PackageName string

func (p PackageName) String() string       { return string(p) }
func (p PackageName) Equals(s string) bool { return string(p) == s }

// Implements fnerrors.Location.
func (p PackageName) ErrorLocation() string { return string(p) }

func Name(str string) PackageName {
	return PackageName(filepath.Clean(str))
}

type PackageList struct {
	l uniquestrings.List
}

func (pl *PackageList) Add(pkg PackageName) bool {
	return pl.l.Add(pkg.String())
}

func (pl *PackageList) AddMultiple(pkgs ...PackageName) {
	for _, pkg := range pkgs {
		pl.l.Add(pkg.String())
	}
}

func (pl PackageList) PackageNames() []PackageName {
	o := pl.PackageNamesAsString()
	r := make([]PackageName, len(o))
	for k, v := range o {
		r[k] = PackageName(v)
	}
	return r
}

func (pl PackageList) PackageNamesAsString() []string {
	return pl.l.Strings()
}

func (pl PackageList) Includes(pkg PackageName) bool {
	return pl.l.Has(pkg.String())
}

func (pl PackageList) Len() int { return pl.l.Len() }

func (pl PackageList) Clone() PackageList {
	return PackageList{l: pl.l.Clone()}
}

func PackageNames(strs ...string) []PackageName {
	o := make([]PackageName, len(strs))
	for k, s := range strs {
		o[k] = PackageName(s)
	}
	return o
}

func Strs(sch ...PackageName) []string {
	o := make([]string, len(sch))
	for k, s := range sch {
		o[k] = s.String()
	}
	return o
}

func IsParent(moduleName string, sch PackageName) (string, bool) {
	if sch.Equals(moduleName) {
		return ".", true
	}
	name := string(sch)
	if rel := strings.TrimPrefix(name, moduleName+"/"); rel != name {
		return rel, true
	}
	return "", false
}

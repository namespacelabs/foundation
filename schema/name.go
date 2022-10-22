// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

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

func MakePackageName(str string) PackageName {
	return PackageName(filepath.Clean(str))
}

type PackageList struct {
	l uniquestrings.List
}

type PackageRefList struct {
	l    uniquestrings.List
	refs []*PackageRef
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

func (pl *PackageRefList) Add(ref *PackageRef) bool {
	if pl.l.Add(ref.Canonical()) {
		pl.refs = append(pl.refs, ref)
		return true
	}

	return false
}

func (pl PackageRefList) Refs() []*PackageRef {
	return pl.refs
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

func List(packages []PackageName) PackageList {
	var pl PackageList
	pl.AddMultiple(packages...)
	return pl
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

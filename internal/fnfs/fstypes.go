// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fnfs

import (
	"io/fs"
	"path/filepath"
	"strings"

	"namespacelabs.dev/foundation/schema"
)

// Identifies a package/file within a module.
type Location struct {
	ModuleName string
	// FS rooted at the module [ModuleName] root.
	FS fs.FS
	// Path within the module (within [FS]).
	RelPath string
}

func (loc Location) Rel(rel ...string) string {
	return filepath.Join(append([]string{loc.RelPath}, rel...)...)
}

func (loc Location) String() string {
	if loc.RelPath == "." {
		return loc.ModuleName
	}
	return filepath.Join(loc.ModuleName, loc.RelPath)
}

func (loc Location) AsPackageName() schema.PackageName {
	return schema.PackageName(loc.String())
}

// Implements the fnerrors.Location interface.
func (loc Location) ErrorLocation() string {
	return loc.RelPath
}

func ResolveLocation(moduleName, packageName string) (Location, bool) {
	if moduleName == packageName {
		return Location{ModuleName: moduleName, RelPath: "."}, true
	} else if x := strings.TrimPrefix(packageName, moduleName+"/"); x != packageName {
		return Location{ModuleName: moduleName, RelPath: x}, true
	}

	return Location{}, false
}

// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package fnfs

import (
	"io/fs"
	"path/filepath"

	"namespacelabs.dev/foundation/schema"
)

type Location struct {
	ModuleName string
	FS         fs.FS
	RelPath    string
}

func (loc Location) Rel(rel ...string) string {
	return filepath.Join(append([]string{loc.RelPath}, rel...)...)
}

func (loc Location) AsPackageName() schema.PackageName {
	if loc.RelPath == "." {
		return schema.PackageName(loc.ModuleName)
	}
	return schema.PackageName(filepath.Join(loc.ModuleName, loc.RelPath))
}

// ErrorLocation implements the fnerrors.Location interface.
func (loc Location) ErrorLocation() string {
	return loc.RelPath
}
// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package dirs

import (
	"golang.org/x/exp/slices"
)

// Any sub-directory.
var DirsToExclude = []string{
	".git",
	".parcel-cache",
	// NodeJS-specific
	"node_modules",
	// Yarn-specific
	".yarn/cache",
	".yarn/unplugged",
}

// Relative to the workspace.
var FilesToExclude = []string{
	// Yarn-specific
	"install-state.gz",
	".pnp.*",
}

func IsExcluded(fullPath string, name string) bool {
	return (len(name) > 1 && name[0] == '.') || slices.Contains(DirsToExclude, name)
}

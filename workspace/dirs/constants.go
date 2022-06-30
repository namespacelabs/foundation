// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package dirs

import (
	"golang.org/x/exp/slices"
)

// Any sub-directory.
var DirsToExclude = []string{".git", ".parcel-cache", "node_modules"}

// Relative to the workspace.
var FilesToExclude = []string{}

func IsExcluded(fullPath string, name string) bool {
	return (len(name) > 1 && name[0] == '.') || slices.Contains(DirsToExclude, name)
}

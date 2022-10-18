// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package dirs

import (
	"golang.org/x/exp/slices"
)

var (
	// Directories to exlude from source scanning.
	// Any sub-directory.
	dirsToExclude = []string{"node_modules", ".history"}

	// Patterns to exclude by default when building images. Integrations
	// (e.g. nodejs) may add additional patterns.
	BasePatternsToExclude = []string{
		"**/node_modules/*",
		"**/.git/*",
		"**/.parcel-cache/*",
	}
)

// Returns false if the directory shouldn't be scanned for Namespace source files (.cue, .proto).
// This doesn't affect the content that is copied to the build image.
func IsExcludedAsSource(name string) bool {
	return slices.Contains(dirsToExclude, name)
}

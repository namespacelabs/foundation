// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package dirs

import (
	"namespacelabs.dev/foundation/internal/fnfs"
)

var (
	// Patterns to exclude by default when building images. Integrations
	// (e.g. nodejs) may add additional patterns.
	BasePatternsToExclude = []string{
		"**/node_modules/*",
		"**/.git/*",
		"**/.parcel-cache/*",
		"**/.yarn/cache/*",
		"**/.history/*",
	}

	ExcludeMatcher *fnfs.PatternMatcher
)

func init() {
	m, err := fnfs.NewMatcher(fnfs.MatcherOpts{ExcludeFilesGlobs: BasePatternsToExclude})
	if err != nil {
		panic(err)
	}
	ExcludeMatcher = m
}

// Returns false if the directory shouldn't be scanned for Namespace source files (.cue, .proto).
// This doesn't affect the content that is copied to the build image.
func IsExcludedAsSource(path string) bool {
	return ExcludeMatcher.Excludes(path)
}

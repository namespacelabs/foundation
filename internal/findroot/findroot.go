// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package findroot

import (
	"fmt"
	"os"
	"path/filepath"
)

type MatcherFunc func(string) bool

func LookForFile(names ...string) MatcherFunc {
	return func(dir string) bool {
		for _, name := range names {
			if fi, err := os.Stat(filepath.Join(dir, name)); err == nil && !fi.IsDir() {
				return true
			}
		}

		return false
	}
}

func Find(label, startAt string, match MatcherFunc) (string, error) {
	dir := filepath.Clean(startAt)

	for {
		if match(dir) {
			return dir, nil
		}

		d := filepath.Dir(dir)
		if d == dir {
			return "", fmt.Errorf("%s: could not determine root from %q", label, startAt)
		}
		dir = d
	}
}

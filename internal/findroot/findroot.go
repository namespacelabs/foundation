// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package findroot

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

type Matcher struct {
	Entname string
	Test    func(fi fs.FileInfo) bool
}

func LookForFile(name string) Matcher {
	return Matcher{
		Entname: name,
		Test:    func(fi fs.FileInfo) bool { return !fi.IsDir() },
	}
}

func Find(startAt string, match Matcher) (string, error) {
	dir := filepath.Clean(startAt)

	for {
		if fi, err := os.Stat(filepath.Join(dir, match.Entname)); err == nil && match.Test(fi) {
			return dir, nil
		}

		d := filepath.Dir(dir)
		if d == dir {
			return "", errors.New("could not determine root")
		}
		dir = d
	}
}
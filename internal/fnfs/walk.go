// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package fnfs

import "io/fs"

func WalkDir(fsys fs.FS, dir string, callback func(string, fs.DirEntry) error) error {
	return fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		return callback(path, d)
	})
}

func WalkDirWithMatcher(fsys fs.FS, dir string, matcher *PatternMatcher, callback func(string, fs.DirEntry) error) error {
	return fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if matcher.Excludes(path) || !matcher.Includes(path) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		return callback(path, d)
	})
}

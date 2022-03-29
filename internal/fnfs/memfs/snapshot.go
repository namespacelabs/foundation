// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package memfs

import (
	"io/fs"
	"sort"

	"github.com/mattn/go-zglob"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

type SnapshotOpts struct {
	IncludeFiles      []string
	IncludeFilesGlobs []string
	ExcludeFilesGlobs []string
}

type SnapshotFS interface {
	Snapshot(SnapshotOpts) (*FS, error)
}

type matcher struct {
	requiredFileMap map[string]bool
	includeGlobs    []hasMatch
	excludeGlobs    []hasMatch
}

func newMatcher(opts SnapshotOpts) (*matcher, error) {
	m := &matcher{}

	if len(opts.IncludeFiles) > 0 {
		m.requiredFileMap = map[string]bool{}
		for _, f := range opts.IncludeFiles {
			m.requiredFileMap[f] = true
		}
	}

	for _, glob := range opts.IncludeFilesGlobs {
		x, err := zglob.New(glob)
		if err != nil {
			return m, err
		}
		m.includeGlobs = append(m.includeGlobs, x)
	}

	for _, glob := range opts.ExcludeFilesGlobs {
		x, err := zglob.New(glob)
		if err != nil {
			return m, err
		}
		m.excludeGlobs = append(m.excludeGlobs, x)
	}

	return m, nil
}

func (m *matcher) excludes(name string) bool {
	for _, m := range m.excludeGlobs {
		if m.Match(name) {
			return true
		}
	}

	return false
}

func (m *matcher) includes(path, name string) bool {
	if m.requiredFileMap != nil {
		if m.requiredFileMap[path] {
			delete(m.requiredFileMap, path)
			return true
		}

		return false
	}

	if len(m.includeGlobs) > 0 {
		for _, glob := range m.includeGlobs {
			if glob.Match(name) {
				return true
			}
		}
		return false
	}

	return true
}

func Snapshot(fsys fs.FS, opts SnapshotOpts) (*FS, error) {
	if snapfs, ok := fsys.(SnapshotFS); ok {
		return snapfs.Snapshot(opts)
	}

	return snapshotWith(fsys, opts, ".", true, func(path string) ([]byte, error) {
		return fs.ReadFile(fsys, path)
	})
}

func SnapshotDir(fsys fs.FS, dir string, opts SnapshotOpts) (*FS, error) {
	return snapshotWith(fsys, opts, dir, false, func(path string) ([]byte, error) {
		return fs.ReadFile(fsys, path)
	})
}

func snapshotWith(fsys fs.FS, opts SnapshotOpts, dir string, godeep bool, readFile func(string) ([]byte, error)) (*FS, error) {
	m, err := newMatcher(opts)
	if err != nil {
		return nil, err
	}

	var snapshot FS
	if err := fs.WalkDir(fsys, dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if m.excludes(d.Name()) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}

		if d.IsDir() {
			if godeep || path == dir {
				return nil
			}
			return fs.SkipDir
		}

		if !m.includes(path, d.Name()) {
			return nil
		}

		contents, err := readFile(path)
		if err != nil {
			return err
		}

		snapshot.Add(path, contents)
		return nil
	}); err != nil {
		return nil, err
	}

	if len(m.requiredFileMap) > 0 {
		var left []string
		for k := range m.requiredFileMap {
			left = append(left, k)
		}
		sort.Strings(left)
		return nil, fnerrors.BadInputError("failed to load required files: %v", left)
	}

	return &snapshot, nil
}

type hasMatch interface {
	Match(name string) bool
}
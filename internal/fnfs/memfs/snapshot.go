// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package memfs

import (
	"io/fs"
	"sort"

	"github.com/mattn/go-zglob"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnerrors/multierr"
)

type SnapshotOpts struct {
	RequireIncludeFiles bool
	IncludeFiles        []string
	IncludeFilesGlobs   []string
	ExcludeFilesGlobs   []string
}

type SnapshotFS interface {
	Snapshot(SnapshotOpts) (*FS, error)
}

type matcher struct {
	requiredFileMap map[string]struct{}
	observedFileMap map[string]struct{}
	includeGlobs    []WithMatch
	excludeGlobs    []WithMatch
}

func newMatcher(opts SnapshotOpts) (*matcher, error) {
	m := &matcher{}

	if len(opts.IncludeFiles) > 0 {
		m.requiredFileMap = map[string]struct{}{}
		for _, f := range opts.IncludeFiles {
			m.requiredFileMap[f] = struct{}{}
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

func ExcludeMatcher(globs []string) (WithMatch, error) {
	var errs []error
	var excludeGlobs []WithMatch
	for _, glob := range globs {
		x, err := zglob.New(glob)
		if err != nil {
			errs = append(errs, err)
		} else {
			excludeGlobs = append(excludeGlobs, x)
		}
	}

	if err := multierr.New(errs...); err != nil {
		return nil, err
	}

	return matchAny{excludeGlobs}, nil
}

type matchAny struct {
	matches []WithMatch
}

func (m matchAny) Match(str string) bool {
	for _, x := range m.matches {
		if x.Match(str) {
			return true
		}
	}
	return false
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
		if _, ok := m.requiredFileMap[path]; ok {
			if m.observedFileMap == nil {
				m.observedFileMap = map[string]struct{}{}
			}
			m.observedFileMap[path] = struct{}{}
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

	if opts.RequireIncludeFiles && len(m.requiredFileMap) != len(m.observedFileMap) {
		var left []string
		for k := range m.requiredFileMap {
			if _, ok := m.observedFileMap[k]; !ok {
				left = append(left, k)
			}
		}
		sort.Strings(left)
		return nil, fnerrors.BadInputError("failed to load required files: %v", left)
	}

	return &snapshot, nil
}

type WithMatch interface {
	Match(name string) bool
}

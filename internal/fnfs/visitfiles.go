// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package fnfs

import (
	"context"
	"io"
	"io/fs"
	"sort"

	"github.com/mattn/go-zglob"
	"namespacelabs.dev/foundation/internal/bytestream"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

type File struct {
	Path     string
	Contents []byte
}

type VisitFS interface {
	VisitFiles(context.Context, func(string, bytestream.ByteStream, fs.DirEntry) error) error
}

func VisitFiles(ctx context.Context, fsys fs.FS, visitor func(string, bytestream.ByteStream, fs.DirEntry) error) error {
	if vfs, ok := fsys.(VisitFS); ok {
		return vfs.VisitFiles(ctx, visitor)
	}

	return fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if err := ctx.Err(); err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		fi, err := d.Info()
		if err != nil {
			return err
		}

		return visitor(path, reader{fsys, path, fi.Size()}, d)
	})
}

type reader struct {
	fsys fs.FS
	path string
	size int64
}

func (b reader) ContentLength() uint64 { return uint64(b.size) }
func (b reader) Reader() (io.ReadCloser, error) {
	return b.fsys.Open(b.path)
}

type VisitFilesOpts struct {
	IncludeFiles      []string
	IncludeFilesGlobs []string
	ExcludeFilesGlobs []string
}

func VisitFilesWithOpts(fsys fs.FS, dir string, opts VisitFilesOpts, callback fs.WalkDirFunc) error {
	m, err := newMatcher(opts)
	if err != nil {
		return err
	}

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
			if path == dir {
				return nil
			}
			return fs.SkipDir
		}

		if !m.includes(path, d.Name()) {
			return nil
		}

		return callback(path, d, nil)
	}); err != nil {
		return err
	}

	if len(m.requiredFileMap) > 0 {
		var left []string
		for k := range m.requiredFileMap {
			left = append(left, k)
		}
		sort.Strings(left)
		return fnerrors.BadInputError("failed to visit required files: %v", left)
	}

	return nil
}

type matcher struct {
	requiredFileMap map[string]bool
	includeGlobs    []HasMatch
	excludeGlobs    []HasMatch
}

func newMatcher(opts VisitFilesOpts) (*matcher, error) {
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

type HasMatch interface {
	Match(name string) bool
}

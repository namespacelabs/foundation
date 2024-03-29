// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fnfs

import (
	"context"
	"io"
	"io/fs"

	"github.com/mattn/go-zglob"
	"namespacelabs.dev/foundation/internal/bytestream"
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

	return WalkDir(fsys, ".", func(path string, d fs.DirEntry) error {
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

type MatcherOpts struct {
	IncludeFiles      []string
	IncludeFilesGlobs []string
	ExcludeFilesGlobs []string
}

type PatternMatcher struct {
	requiredFileMap map[string]struct{}
	includeGlobs    []HasMatch
	excludeGlobs    []HasMatch
}

func NewMatcher(opts MatcherOpts) (*PatternMatcher, error) {
	m := &PatternMatcher{}

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

func (m *PatternMatcher) Excludes(name string) bool {
	for _, m := range m.excludeGlobs {
		if m.Match(name) {
			return true
		}
	}

	return false
}

func (m *PatternMatcher) Includes(name string) bool {
	if _, ok := m.requiredFileMap[name]; ok {
		return true
	}

	for _, glob := range m.includeGlobs {
		if glob.Match(name) {
			return true
		}
	}

	return len(m.requiredFileMap) == 0 && len(m.includeGlobs) == 0
}

type HasMatch interface {
	Match(name string) bool
}

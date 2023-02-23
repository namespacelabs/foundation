// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package memfs

import (
	"context"
	"io/fs"
	"sort"

	"github.com/moby/patternmatcher"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/std/tasks"
)

type SnapshotOpts struct {
	RequireIncludeFiles bool
	IncludeFiles        []string
	ExcludePatterns     []string
}

type SnapshotFS interface {
	Snapshot(SnapshotOpts) (*FS, error)
}

type matcher struct {
	requiredFileMap map[string]struct{}
	observedFileMap map[string]struct{}
	excludeMatcher  *patternmatcher.PatternMatcher
}

func newMatcher(opts SnapshotOpts) (*matcher, error) {
	m := &matcher{}

	if len(opts.IncludeFiles) > 0 {
		m.requiredFileMap = map[string]struct{}{}
		for _, f := range opts.IncludeFiles {
			m.requiredFileMap[f] = struct{}{}
		}
	}

	if len(opts.ExcludePatterns) > 0 {
		excludeMatcher, err := patternmatcher.New(opts.ExcludePatterns)
		if err != nil {
			return nil, err
		}
		m.excludeMatcher = excludeMatcher
	}

	return m, nil
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

func (m *matcher) includes(file string) (bool, error) {
	if m.requiredFileMap != nil {
		if _, ok := m.requiredFileMap[file]; ok {
			if m.observedFileMap == nil {
				m.observedFileMap = map[string]struct{}{}
			}
			m.observedFileMap[file] = struct{}{}
			return true, nil
		}

		return false, nil
	}

	if m.excludeMatcher != nil {
		excluded, err := m.excludeMatcher.MatchesOrParentMatches(file)
		return !excluded, err
	}

	return true, nil
}

func DeferSnapshot(fsys fs.FS, opts SnapshotOpts) compute.Computable[fs.FS] {
	return compute.Inline(tasks.Action("fs.snapshot"), func(ctx context.Context) (fs.FS, error) {
		return Snapshot(fsys, opts)
	})
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
	if err := fnfs.WalkDir(fsys, dir, func(path string, d fs.DirEntry) error {
		if d.IsDir() {
			if godeep || path == dir {
				return nil
			}
			return fs.SkipDir
		}

		if !d.Type().IsRegular() {
			return nil
		}

		// Only check files for inclusion.
		// A directory might be excluded by default, but files in it may be included.
		if included, err := m.includes(path); err != nil {
			return err
		} else if !included {
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

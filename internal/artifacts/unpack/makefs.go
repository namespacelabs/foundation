// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package unpack

import (
	"context"
	"io"
	"io/fs"
	"math"
	"path/filepath"

	"namespacelabs.dev/foundation/internal/artifacts"
	"namespacelabs.dev/foundation/internal/artifacts/download"
	"namespacelabs.dev/foundation/internal/bytestream"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/std/tasks"
)

func MakeFilesystem(filename string, mode fs.FileMode, ref artifacts.Reference) compute.Computable[fs.FS] {
	return &makeFS{ref: ref, dirent: memfs.FileDirent{Path: filename, FileMode: mode}, contents: download.URL(ref)}
}

type makeFS struct {
	ref artifacts.Reference // Presentation only.

	dirent   memfs.FileDirent
	contents compute.Computable[bytestream.ByteStream]

	compute.LocalScoped[fs.FS]
}

var _ compute.Computable[fs.FS] = &makeFS{}

func (u *makeFS) Action() *tasks.ActionEvent {
	return tasks.Action("download.make-fs").Arg("url", u.ref.URL)
}
func (u *makeFS) Inputs() *compute.In {
	return compute.Inputs().JSON("dirent", u.dirent).Computable("contents", u.contents)
}

func (u *makeFS) Compute(ctx context.Context, deps compute.Resolved) (fs.FS, error) {
	contents := compute.MustGetDepValue(deps, u.contents, "contents")
	if contents.ContentLength() > math.MaxInt64 {
		return nil, fnerrors.InternalError("file is too large") // Doesn't fit fs.FileInfo
	}
	d := u.dirent
	d.ContentSize = int64(contents.ContentLength())
	return singleFileFS{d, contents}, nil
}

type singleFileFS struct {
	dirent   memfs.FileDirent
	contents bytestream.ByteStream
}

var _ fnfs.VisitFS = singleFileFS{}

func (fsys singleFileFS) Open(name string) (fs.File, error) {
	if filepath.Clean(name) != fsys.dirent.Path {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}

	r, err := fsys.contents.Reader()
	if err != nil {
		return nil, err
	}

	return singleFile{dirent: fsys.dirent, ReadCloser: r}, nil
}

func (fsys singleFileFS) VisitFiles(ctx context.Context, f func(string, bytestream.ByteStream, fs.DirEntry) error) error {
	return f(fsys.dirent.Path, fsys.contents, fsys.dirent)
}

type singleFile struct {
	io.ReadCloser
	dirent memfs.FileDirent
}

func (f singleFile) Stat() (fs.FileInfo, error) {
	return f.dirent, nil
}

// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fnfs

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"namespacelabs.dev/foundation/internal/fnerrors"
)

type LocalFS interface {
	fs.ReadDirFS
	WriteFS
	MkdirFS
}

func Local(path string, rel ...string) LocalFS {
	resolvedPath := filepath.Join(append([]string{path}, rel...)...)

	return makeLocal(&local{root: resolvedPath, readWrite: false})
}

func ReadWriteLocalFS(path string, opts ...LocalOpt) LocalFS {
	return makeLocal(&local{root: path, readWrite: true}, opts...)
}

type LocalOpt interface {
	apply(*local)
}

func AnnounceWrites(to io.Writer) LocalOpt {
	return announceWrites{to}
}

type announceWrites struct{ to io.Writer }

func (aw announceWrites) apply(l *local) {
	l.announceWritesTo = aw.to
}

func makeLocal(local *local, opts ...LocalOpt) *local {
	for _, opt := range opts {
		opt.apply(local)
	}
	return local
}

type local struct {
	root      string
	readWrite bool

	announceWritesTo io.Writer
}

var _ MkdirFS = local{}
var _ RmdirFS = local{}
var _ fs.SubFS = local{}

func (l local) Open(path string) (fs.File, error) {
	return os.DirFS(l.root).Open(path)
}

func (l local) ReadDir(path string) ([]fs.DirEntry, error) {
	if !fs.ValidPath(path) {
		return nil, &fs.PathError{Op: "readdir", Path: path, Err: fnerrors.Newf("invalid name")}
	}

	return os.ReadDir(filepath.Join(l.root, path))
}

func (l local) OpenWrite(path string, mode fs.FileMode) (WriteFileHandle, error) {
	if !l.readWrite {
		return nil, &fs.PathError{Op: "write", Path: path, Err: fnerrors.Newf("fs is read-only")}
	}

	if !fs.ValidPath(path) {
		return nil, &fs.PathError{Op: "write", Path: path, Err: fnerrors.Newf("invalid name")}
	}

	f, err := os.OpenFile(filepath.Join(l.root, path), os.O_CREATE|os.O_WRONLY|os.O_TRUNC|os.O_SYNC, mode)
	if err != nil {
		return nil, err
	}

	// XXX this is not quite correct, we should output the path _after_ the write.
	if l.announceWritesTo != nil {
		fmt.Fprintln(l.announceWritesTo, path)
	}

	return f, nil
}

func (l local) Remove(path string) error {
	if !fs.ValidPath(path) {
		return &fs.PathError{Op: "remove", Path: path, Err: fnerrors.Newf("invalid name")}
	}

	return os.Remove(filepath.Join(l.root, path))
}

func (l local) RemoveAll(path string) error {
	if !fs.ValidPath(path) {
		return &fs.PathError{Op: "removeAll", Path: path, Err: fnerrors.Newf("invalid name")}
	}

	return os.RemoveAll(filepath.Join(l.root, path))
}

func (l local) MkdirAll(path string, mode fs.FileMode) error {
	if !fs.ValidPath(path) {
		return &fs.PathError{Op: "mkdir", Path: path, Err: fnerrors.Newf("invalid name")}
	}

	return os.MkdirAll(filepath.Join(l.root, path), mode)
}

func (l local) Sub(dir string) (fs.FS, error) {
	if strings.HasPrefix(dir, "../") {
		return nil, fnerrors.Newf("can't escape the local workspace")
	}

	return &local{
		root:             filepath.Join(l.root, dir),
		readWrite:        l.readWrite,
		announceWritesTo: l.announceWritesTo,
	}, nil
}

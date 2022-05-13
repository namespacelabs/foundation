// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package fnfs

import (
	"context"
	"io"
	"io/fs"

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

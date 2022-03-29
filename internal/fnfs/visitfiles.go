// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package fnfs

import (
	"context"
	"io/fs"
)

type File struct {
	Path     string
	Contents []byte
}

type VisitFS interface {
	VisitFiles(context.Context, func(string, []byte, fs.DirEntry) error) error
}

func VisitFiles(ctx context.Context, fsys fs.FS, visitor func(string, []byte, fs.DirEntry) error) error {
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

		contents, err := fs.ReadFile(fsys, path)
		if err != nil {
			return err
		}

		return visitor(path, contents, d)
	})
}

func AllFiles(ctx context.Context, fsys fs.ReadDirFS) ([]File, error) {
	var files []File
	return files, VisitFiles(ctx, fsys, func(path string, contents []byte, dirent fs.DirEntry) error {
		files = append(files, File{path, contents})
		return nil
	})
}
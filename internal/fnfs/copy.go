// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fnfs

import (
	"context"
	"io"
	"io/fs"
	"path/filepath"

	"namespacelabs.dev/foundation/internal/bytestream"
)

func CopyTo(ctx context.Context, dst WriteFS, dstBasePath string, src fs.FS) error {
	return VisitFiles(ctx, src, func(path string, contents bytestream.ByteStream, dirent fs.DirEntry) error {
		st, err := dirent.Info()
		if err != nil {
			return err
		}

		target := filepath.Join(dstBasePath, path)

		if mkdir, has := dst.(MkdirFS); has {
			dir := filepath.Dir(target)
			// Not using as we may not have write permissions then: addExecToRead(st.Mode())
			if err := mkdir.MkdirAll(dir, 0700); err != nil {
				return err
			}
		}

		return WriteByteStream(ctx, dst, target, contents, st.Mode().Perm())
	})
}

func CopyFile(dst WriteFS, dstFile string, src fs.FS, srcFile string) error {
	r, err := src.Open(srcFile)
	if err != nil {
		return err
	}
	defer r.Close()

	st, err := r.Stat()
	if err != nil {
		return err
	}

	if mkdir, has := dst.(MkdirFS); has {
		dir := filepath.Dir(dstFile)
		if err := mkdir.MkdirAll(dir, addExecToRead(st.Mode())); err != nil {
			return err
		}
	}

	w, err := dst.OpenWrite(dstFile, st.Mode())
	if err != nil {
		return err
	}
	defer w.Close()

	_, err = io.Copy(w, r)
	return err
}

func addExecToRead(mode fs.FileMode) fs.FileMode {
	if (mode & 0004) != 0 {
		mode |= 0001
	}
	if (mode & 0040) != 0 {
		mode |= 0010
	}
	if (mode & 0400) != 0 {
		mode |= 0100
	}
	return mode
}

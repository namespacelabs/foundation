// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fnfs

import (
	"bytes"
	"context"
	"io"
	"io/fs"

	"namespacelabs.dev/foundation/internal/bytestream"
	"namespacelabs.dev/foundation/internal/ctxio"
)

type WriteFS interface {
	OpenWrite(path string, mode fs.FileMode) (WriteFileHandle, error)
	Remove(path string) error
}

type MkdirFS interface {
	MkdirAll(path string, mode fs.FileMode) error
}

type RmdirFS interface {
	RemoveAll(path string) error
}

type ChmodFS interface {
	Chmod(path string, mode fs.FileMode) error
}

type WriteFileHandle interface {
	io.WriteCloser
}

func WriteFile(ctx context.Context, fs WriteFS, path string, contents []byte, mode fs.FileMode) error {
	w, err := fs.OpenWrite(path, mode.Perm())
	if err != nil {
		return err
	}

	_, writeErr := io.Copy(ctxio.WriterWithContext(ctx, w, nil), bytes.NewReader(contents))
	closeErr := w.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

func WriteByteStream(ctx context.Context, fs WriteFS, path string, contents bytestream.ByteStream, mode fs.FileMode) error {
	r, err := contents.Reader()
	if err != nil {
		return err
	}

	defer r.Close()

	w, err := fs.OpenWrite(path, mode.Perm())
	if err != nil {
		return err
	}

	_, writeErr := io.Copy(ctxio.WriterWithContext(ctx, w, nil), r)
	closeErr := w.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

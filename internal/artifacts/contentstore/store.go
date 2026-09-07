// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package contentstore

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/go-ids"
)

type Info interface {
	Size() int64
}

type ReaderAtCloser interface {
	io.ReaderAt
	io.ReadCloser
}

type Store interface {
	Bytes(context.Context, schema.Digest) ([]byte, error)
	Blob(schema.Digest) (ReaderAtCloser, error)
	Stat(context.Context, schema.Digest) (Info, error)
	WriteBlob(context.Context, schema.Digest, io.ReadCloser) error
	WriteBytes(context.Context, schema.Digest, []byte) error
}

func NewTemporary() (Store, func(), error) {
	root, err := os.MkdirTemp("", "foundation-artifacts-")
	if err != nil {
		return nil, nil, err
	}

	dir := filepath.Join(root, "blobs")
	if err := os.MkdirAll(filepath.Join(dir, "sha256"), 0700|os.ModeDir); err != nil {
		os.RemoveAll(root)
		return nil, nil, err
	}

	return &store{path: dir}, func() { os.RemoveAll(root) }, nil
}

type store struct {
	path string
}

func (s *store) blobPath(d schema.Digest) string {
	return filepath.Join(s.path, d.Algorithm, d.Hex)
}

func (s *store) Bytes(_ context.Context, d schema.Digest) ([]byte, error) {
	if !d.IsSet() {
		return nil, fnerrors.InternalError("digest not set")
	}
	return os.ReadFile(s.blobPath(d))
}

func (s *store) Blob(d schema.Digest) (ReaderAtCloser, error) {
	if !d.IsSet() {
		return nil, fnerrors.InternalError("digest not set")
	}
	return os.Open(s.blobPath(d))
}

func (s *store) Stat(_ context.Context, d schema.Digest) (Info, error) {
	return os.Stat(s.blobPath(d))
}

func (s *store) WriteBytes(ctx context.Context, d schema.Digest, contents []byte) error {
	return s.WriteBlob(ctx, d, io.NopCloser(bytes.NewReader(contents)))
}

func (s *store) WriteBlob(_ context.Context, d schema.Digest, contents io.ReadCloser) error {
	defer contents.Close()

	path := s.blobPath(d)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	tmp := filepath.Join(s.path, d.Algorithm, fmt.Sprintf(".%s.%s", d.Hex, ids.NewRandomBase32ID(8)))
	w, err := os.Create(tmp)
	if err != nil {
		return err
	}

	_, copyErr := io.Copy(w, contents)
	closeErr := w.Close()
	if copyErr != nil {
		os.Remove(tmp)
		return copyErr
	}
	if closeErr != nil {
		os.Remove(tmp)
		return closeErr
	}
	return os.Rename(tmp, path)
}

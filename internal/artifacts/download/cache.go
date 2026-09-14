// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package download

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
	"github.com/opencontainers/go-digest"
	"namespacelabs.dev/foundation/internal/bytestream"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
)

func (dl *downloadUrl) downloadCached(ctx context.Context) (bytestream.ByteStream, error) {
	if dl.digest == nil {
		return nil, fnerrors.InternalError("cached download requires a digest")
	}
	if err := digest.Digest(dl.digest.String()).Validate(); err != nil {
		return nil, err
	}

	dir, err := dirs.Ensure(dirs.Subdir(filepath.Join("sdk-downloads", dl.digest.Algorithm)))
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, dl.digest.Hex)
	lock := flock.New(path + ".lock")
	locked, err := lock.TryLockContext(ctx, 100*time.Millisecond)
	if err != nil {
		return nil, err
	}
	if !locked {
		return nil, ctx.Err()
	}
	defer lock.Close()

	if info, err := os.Stat(path); err == nil {
		cached := cachedFile{path: path, size: uint64(info.Size())}
		got, err := bytestream.Digest(ctx, cached)
		if err == nil && got.Equals(*dl.digest) {
			return cached, nil
		}
	}

	contents, err := dl.download(ctx)
	if err != nil {
		return nil, err
	}

	// Publish only complete, verified downloads; interrupted writes must not
	// become cache hits in subsequent commands.
	tmp, err := os.CreateTemp(dir, ".download-*")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp.Name())
	writeErr := bytestream.WriteTo(tmp, contents)
	closeErr := tmp.Close()
	if writeErr != nil {
		return nil, writeErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return nil, err
	}
	return cachedFile{path: path, size: contents.ContentLength()}, nil
}

type cachedFile struct {
	path string
	size uint64
}

func (f cachedFile) ContentLength() uint64          { return f.size }
func (f cachedFile) Reader() (io.ReadCloser, error) { return os.Open(f.path) }

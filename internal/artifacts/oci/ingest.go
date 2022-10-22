// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package oci

import (
	"compress/gzip"
	"context"
	"io"
	"io/fs"

	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"namespacelabs.dev/foundation/internal/artifacts"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/std/tasks"
)

func IngestFromFS(ctx context.Context, fsys fs.FS, path string, compressed bool) (Image, error) {
	img, err := tarball.Image(func() (io.ReadCloser, error) {
		f, err := fsys.Open(path)
		if err != nil {
			return nil, err
		}

		fi, err := f.Stat()
		if err != nil {
			return nil, fnerrors.InternalError("failed to stat intermediate image: %w", err)
		}

		progress := artifacts.NewProgressReader(f, uint64(fi.Size()))
		tasks.Attachments(ctx).SetProgress(progress)

		if !compressed {
			return progress, nil
		}

		gr, err := gzip.NewReader(progress)
		if err != nil {
			return nil, err
		}

		return andClose{gr, progress}, nil
	}, nil)
	if err != nil {
		return nil, err
	}

	return Canonical(ctx, img)
}

type andClose struct {
	actual io.ReadCloser
	closer io.Closer
}

func (a andClose) Read(p []byte) (int, error) { return a.actual.Read(p) }
func (a andClose) Close() error {
	err := a.actual.Close()
	ioerr := a.closer.Close()
	if err != nil {
		return err
	}
	return ioerr
}

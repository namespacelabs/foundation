// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package oci

import (
	"bytes"
	"context"
	"io"
	"io/fs"
	"io/ioutil"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"namespacelabs.dev/foundation/internal/fnfs/maketarfs"
)

type HasToLayer interface {
	AsLayer() (v1.Layer, error)
}

func LayerFromFS(ctx context.Context, vfs fs.FS) (Layer, error) {
	if asL, ok := vfs.(HasToLayer); ok {
		return asL.AsLayer()
	}

	var buf bytes.Buffer

	if err := maketarfs.TarFS(ctx, &buf, vfs, nil, nil); err != nil {
		return nil, err
	}

	return tarball.LayerFromOpener(func() (io.ReadCloser, error) {
		return ioutil.NopCloser(bytes.NewReader(buf.Bytes())), nil
	}, tarball.WithCompressedCaching)
}
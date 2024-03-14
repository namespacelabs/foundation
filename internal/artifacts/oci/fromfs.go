// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package oci

import (
	"bytes"
	"context"
	"io"
	"io/fs"
	"os"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/klauspost/compress/zstd"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnfs/maketarfs"
	"namespacelabs.dev/foundation/std/tasks"
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
		return io.NopCloser(bytes.NewReader(buf.Bytes())), nil
	}, tarball.WithCompressedCaching)
}

func LayerFromFile(description string, vfs fs.FS, path string) NamedLayer {
	return MakeNamedLayer(description, &loadLayer{vfs: vfs, path: path})
}

type loadLayer struct {
	vfs  fs.FS
	path string

	compute.LocalScoped[Layer]
}

func (ll *loadLayer) Action() *tasks.ActionEvent {
	return tasks.Action("oci.load-layer-from-fs").Arg("path", ll.path)
}
func (ll *loadLayer) Inputs() *compute.In {
	return compute.Inputs().Indigestible("vfs", ll.vfs).Str("path", ll.path)
}
func (ll *loadLayer) Output() compute.Output { return compute.Output{NotCacheable: true} }
func (ll *loadLayer) Compute(ctx context.Context, _ compute.Resolved) (Layer, error) {
	return tarball.LayerFromOpener(func() (io.ReadCloser, error) {
		return ll.vfs.Open(ll.path)
	}, tarball.WithCompressedCaching)
}

func RawZstdLayerFrom(file string, mediaType types.MediaType) (Layer, error) {
	rl := rawLayer{localFile: file, mediaType: mediaType}

	chash, csize, err := computeDigest(rl.Compressed)
	if err != nil {
		return nil, err
	}

	uhash, _, err := computeDigest(rl.Uncompressed)
	if err != nil {
		return nil, err
	}

	rl.cdigest = chash
	rl.csize = csize
	rl.udigest = uhash

	return rl, nil
}

type rawLayer struct {
	localFile string
	cdigest   v1.Hash
	csize     int64
	udigest   v1.Hash
	mediaType types.MediaType
}

// Digest returns the Hash of the compressed layer.
func (r rawLayer) Digest() (v1.Hash, error) {
	return r.cdigest, nil
}

// DiffID returns the Hash of the uncompressed layer.
func (r rawLayer) DiffID() (v1.Hash, error) {
	return r.udigest, nil
}

// Compressed returns an io.ReadCloser for the compressed layer contents.
func (r rawLayer) Compressed() (io.ReadCloser, error) {
	return os.Open(r.localFile)
}

// Uncompressed returns an io.ReadCloser for the uncompressed layer contents.
func (r rawLayer) Uncompressed() (io.ReadCloser, error) {
	u, err := r.Compressed()
	if err != nil {
		return nil, err
	}

	reader, err := zstd.NewReader(u)
	if err != nil {
		return nil, err
	}

	return closeBoth{reader, u}, nil
}

type closeBoth struct {
	io.Reader

	source io.ReadCloser
}

func (cl closeBoth) Close() error {
	return cl.source.Close()
}

// Size returns the compressed size of the Layer.
func (r rawLayer) Size() (int64, error) {
	return r.csize, nil
}

// MediaType returns the media type of the Layer.
func (r rawLayer) MediaType() (types.MediaType, error) { return r.mediaType, nil }

func computeDigest(opener tarball.Opener) (v1.Hash, int64, error) {
	rc, err := opener()
	if err != nil {
		return v1.Hash{}, 0, err
	}
	defer rc.Close()

	return v1.SHA256(rc)
}

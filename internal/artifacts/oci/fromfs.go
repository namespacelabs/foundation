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
	"namespacelabs.dev/foundation/engine/compute"
	"namespacelabs.dev/foundation/internal/fnfs/maketarfs"
	"namespacelabs.dev/foundation/workspace/tasks"
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

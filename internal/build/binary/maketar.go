// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package binary

import (
	"compress/gzip"
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/build"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type makeTarImage struct {
	spec     build.Spec
	target   string
	compress bool
}

func (m makeTarImage) BuildImage(ctx context.Context, env pkggraph.SealedContext, conf build.Configuration) (compute.Computable[oci.Image], error) {
	inner, err := m.spec.BuildImage(ctx, env, conf)
	if err != nil {
		return nil, err
	}

	return compute.Transform("binary.make_tar", inner, func(ctx context.Context, img oci.Image) (oci.Image, error) {
		dir, err := os.MkdirTemp("", "squashfs")
		if err != nil {
			return nil, err
		}

		x := filepath.Join(dir, m.target)

		if err := os.MkdirAll(filepath.Dir(x), 0755); err != nil {
			return nil, err
		}

		defer os.RemoveAll(dir)

		f, err := os.Create(x)
		if err != nil {
			return nil, err
		}

		defer f.Close()

		r := mutate.Extract(img)
		defer r.Close()

		var w io.WriteCloser = f
		if m.compress {
			gw, err := gzip.NewWriterLevel(f, 6)
			if err != nil {
				return nil, err
			}
			w = gw
		}

		if _, err := io.Copy(w, r); err != nil {
			return nil, err
		}

		if err := w.Close(); err != nil {
			return nil, err
		}

		if w != f {
			if err := f.Close(); err != nil {
				return nil, err
			}
		}

		layer, err := oci.LayerFromFS(ctx, os.DirFS(dir))
		if err != nil {
			return nil, err
		}

		return mutate.AppendLayers(empty.Image, layer)
	}), nil
}

func (m makeTarImage) PlatformIndependent() bool { return m.spec.PlatformIndependent() }

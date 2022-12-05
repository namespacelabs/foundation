// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package buildkit

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/moby/buildkit/client"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
	"namespacelabs.dev/foundation/std/tasks"
)

const KeySourceDateEpoch = "source-date-epoch"

type exporter[V any] interface {
	Prepare(context.Context) error
	Exports() []client.ExportEntry
	Provide(context.Context, *client.SolveResponse, clientOpts) (V, error)
}

func exportToImage(opts clientOpts) exporter[oci.Image] {
	if opts.SupportsCanonicalBuilds {
		return &exportOCILayout{}
	}

	return &exportImage{}
}

type exportImage struct {
	output *os.File
}

func (e *exportImage) Prepare(ctx context.Context) error {
	f, err := dirs.CreateUserTemp("buildkit", "image")
	if err != nil {
		return err
	}

	// ExportEntry below takes care of closing f.
	e.output = f

	compute.On(ctx).Cleanup(tasks.Action("buildkit.build-image.cleanup").Arg("name", f.Name()), func(ctx context.Context) error {
		return os.Remove(f.Name())
	})

	return nil
}

func (e *exportImage) Exports() []client.ExportEntry {
	return []client.ExportEntry{{
		Type: client.ExporterOCI,
		Attrs: map[string]string{
			"buildinfo":        "false", // Remove build info to keep reproducibility.
			KeySourceDateEpoch: "0",
		},
		Output: func(_ map[string]string) (io.WriteCloser, error) {
			return e.output, nil
		},
	}}
}

func (e *exportImage) Provide(ctx context.Context, _ *client.SolveResponse, opts clientOpts) (oci.Image, error) {
	image, err := oci.IngestFromFS(ctx, fnfs.Local(filepath.Dir(e.output.Name())), filepath.Base(e.output.Name()), false)
	if err != nil {
		return nil, err
	}

	if opts.SupportsCanonicalBuilds {
		return image, nil
	}

	return oci.Canonical(ctx, image)
}

type exportOCILayout struct {
	output string
}

func (e *exportOCILayout) Prepare(ctx context.Context) error {
	f, err := dirs.CreateUserTempDir("buildkit", "image")
	if err != nil {
		return err
	}

	// ExportEntry below takes care of closing f.
	e.output = f

	compute.On(ctx).Cleanup(tasks.Action("buildkit.build-image.cleanup").Arg("dir", f), func(ctx context.Context) error {
		return os.RemoveAll(f)
	})

	return nil
}

func (e *exportOCILayout) Exports() []client.ExportEntry {
	return []client.ExportEntry{{
		Type: client.ExporterOCI,
		Attrs: map[string]string{
			"buildinfo":        "false", // Remove build info to keep reproducibility.
			"tar":              "false",
			KeySourceDateEpoch: "0",
		},
		OutputDir: e.output,
	}}
}

func (e *exportOCILayout) Provide(ctx context.Context, _ *client.SolveResponse, _ clientOpts) (oci.Image, error) {
	index, err := layout.ImageIndexFromPath(e.output)
	if err != nil {
		return nil, err
	}

	idx, err := index.IndexManifest()
	if err != nil {
		return nil, err
	}

	if len(idx.Manifests) != 1 {
		return nil, fnerrors.InternalError("buildkit: expected a single image, saw %d", len(idx.Manifests))
	}

	img, err := index.Image(idx.Manifests[0].Digest)
	if err != nil {
		return nil, err
	}

	oci.AttachDigestToAction(ctx, img)
	return img, nil
}

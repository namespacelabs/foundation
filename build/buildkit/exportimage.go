// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package buildkit

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/moby/buildkit/client"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/foundation/workspace/dirs"
)

type exporter[V any] interface {
	Prepare(context.Context) error
	Exports() []client.ExportEntry
	Provide(context.Context, *client.SolveResponse) (V, error)
}

func exportToImage() exporter[oci.Image] { return &exportImage{} }

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
			"buildinfo": "false", // Remove build info to keep reproducibility.
		},
		Output: func(_ map[string]string) (io.WriteCloser, error) {
			return e.output, nil
		},
	}}
}

func (e *exportImage) Provide(ctx context.Context, _ *client.SolveResponse) (oci.Image, error) {
	return oci.IngestFromFS(ctx, fnfs.Local(filepath.Dir(e.output.Name())), filepath.Base(e.output.Name()), false)
}

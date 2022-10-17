// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package binary

import (
	"context"

	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/tasks"
)

type snapshotFiles struct {
	rel   string
	globs []string
}

func (m snapshotFiles) BuildImage(ctx context.Context, env pkggraph.SealedContext, conf build.Configuration) (compute.Computable[oci.Image], error) {
	w := conf.Workspace().VersionedFS(m.rel, true)

	return compute.Map(tasks.Action("binary.snapshot-files"),
		compute.Inputs().Computable("fsys", w),
		compute.Output{},
		func(ctx context.Context, r compute.Resolved) (oci.Image, error) {
			y := compute.MustGetDepValue(r, w, "fsys").FS()

			result, err := memfs.Snapshot(y, memfs.SnapshotOpts{IncludeFiles: m.globs, RequireIncludeFiles: true})
			if err != nil {
				return nil, err
			}

			layer, err := oci.LayerFromFS(ctx, result)
			if err != nil {
				return nil, err
			}

			return mutate.AppendLayers(empty.Image, layer)
		}), nil
}

func (m snapshotFiles) PlatformIndependent() bool { return true }

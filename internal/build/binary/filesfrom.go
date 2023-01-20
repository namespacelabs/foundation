// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package binary

import (
	"context"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/build"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type filesFrom struct {
	spec      build.Spec
	files     []string
	targetDir string
}

func (m filesFrom) BuildImage(ctx context.Context, env pkggraph.SealedContext, conf build.Configuration) (compute.Computable[oci.Image], error) {
	inner, err := m.spec.BuildImage(ctx, env, conf)
	if err != nil {
		return nil, err
	}

	return compute.Transform("binary.files_from", inner, func(ctx context.Context, img oci.Image) (oci.Image, error) {
		fsys := oci.ImageAsFS(img)

		var target memfs.FS
		for _, path := range m.files {
			if err := fnfs.CopyFile(&target, filepath.Join(m.targetDir, path), fsys, path); err != nil {
				return nil, err
			}
		}

		layer, err := oci.LayerFromFS(ctx, &target)
		if err != nil {
			return nil, err
		}

		return mutate.AppendLayers(empty.Image, layer)
	}), nil
}

func (m filesFrom) PlatformIndependent() bool { return m.spec.PlatformIndependent() }

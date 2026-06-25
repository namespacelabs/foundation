// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package binary

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/moby/buildkit/client/llb"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/build"
	"namespacelabs.dev/foundation/internal/build/buildkit"
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
	// Fast path: when the source is a prebuilt image, perform the copy inside
	// BuildKit so the source layers are fetched server-side. This avoids
	// streaming the source image through the client, and lets the resulting
	// image be direct-pushed when conf.PublishName() is set.
	if imgid, ok := build.IsPrebuilt(m.spec); ok && conf.TargetPlatform() != nil {
		return m.buildViaBuildkit(ctx, env, conf, imgid)
	}

	// Fallback: legacy in-memory copy. Required when the source is not a
	// fixed image reference (e.g. another binary in the workspace), or when
	// the build target has no platform.
	return m.buildInMemory(ctx, env, conf)
}

func (m filesFrom) buildViaBuildkit(ctx context.Context, env pkggraph.SealedContext, conf build.Configuration, imgid oci.ImageID) (compute.Computable[oci.Image], error) {
	src := llb.Image(imgid.RepoAndDigest(),
		llb.WithCustomNamef("files_from(%s)", imgid))

	state := llb.Scratch()
	for _, path := range m.files {
		dest := filepath.Join(m.targetDir, path)
		state = state.File(
			llb.Copy(src, path, dest, &llb.CopyInfo{
				CreateDestPath: true,
				AllowWildcard:  true,
			}),
			llb.WithCustomNamef("COPY %s -> %s", path, dest),
		)
	}

	return buildkit.BuildImage(ctx,
		buildkit.DeferClient(env.Configuration(), conf.TargetPlatform()),
		conf,
		state)
}

func (m filesFrom) buildInMemory(ctx context.Context, env pkggraph.SealedContext, conf build.Configuration) (compute.Computable[oci.Image], error) {
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

func (m filesFrom) Description() string {
	return fmt.Sprintf("filesFrom(%s, ...)", m.spec.Description())
}

// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package genbinary

import (
	"context"
	"io/fs"

	"github.com/moby/buildkit/client/llb"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/build"
	"namespacelabs.dev/foundation/internal/build/buildkit"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/dependencies/pins"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/llbutil"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/pkggraph"
)

func NixImageBuilder(packageName schema.PackageName, module *pkggraph.Module, sources fs.FS) build.Spec {
	return nixImage{packageName, sources}
}

type nixImage struct {
	packageName schema.PackageName
	sources     fs.FS
}

func (l nixImage) BuildImage(ctx context.Context, env pkggraph.SealedContext, conf build.Configuration) (compute.Computable[oci.Image], error) {
	return NixImage(ctx, env.Configuration(), conf, l.sources)

}

func (l nixImage) PlatformIndependent() bool { return false }

func NixImage(ctx context.Context, conf cfg.Configuration, target build.BuildTarget, sources fs.FS) (compute.Computable[oci.Image], error) {
	if target.TargetPlatform() == nil {
		return nil, fnerrors.BadInputError("nix: target platform is missing")
	}

	const dirTarget = "/source"
	const outputImageFile = "image.tgz"

	var err error
	sourceFiles, err := llbutil.WriteFS(ctx, sources, llb.Scratch(), "/")
	if err != nil {
		return nil, err
	}

	nixosImage, err := pins.CheckImage("nixos/nix:2.6.0")
	if err != nil {
		return nil, err
	}

	// Filter-syscalls is necessary due to an interaction with Docker, see https://github.com/NixOS/nix/issues/5258
	const nixconf = `# Namespace-managed nix configuration.
build-users-group = nixbld
sandbox = false
trusted-public-keys = cache.nixos.org-1:6NCHdD59X431o0gWypbMrAURkbJ16ZPMQFGspcDShjY=
experimental-features = nix-command flakes
filter-syscalls = false
	`

	base := llb.Image(nixosImage, llb.Platform(*target.TargetPlatform())).
		File(llb.Mkfile("/etc/nix/nix.conf", 0777, []byte(nixconf))).
		AddEnv("PATH", "/root/.nix-profile/bin")

	build := base.
		Run(llb.Shlexf("nix build %s -o /tmp/result", dirTarget))
	build.AddMount("/source", sourceFiles)

	// nix build produces a symlink to the result, which we then need to copy into the target mount so buildkit copies it out.
	postCopy := build.Root().
		Run(llb.Shlexf("cp -L /tmp/result /out/" + outputImageFile))
	out := postCopy.AddMount("/out", llb.Scratch())

	fsys, err := buildkit.BuildFilesystem(ctx, conf, target, out)
	if err != nil {
		return nil, err
	}

	return compute.Transform("ingest generated image", fsys, func(ctx context.Context, fsys fs.FS) (oci.Image, error) {
		return oci.IngestFromFS(ctx, fsys, outputImageFile, true)
	}), nil
}

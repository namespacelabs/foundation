// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package genbinary

import (
	"context"
	"io/fs"

	"github.com/moby/buildkit/client/llb"
	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/build/buildkit"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/llbutil"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/pins"
)

func NixImage(packageName schema.PackageName, module *workspace.Module, sources fs.FS) build.Spec {
	return nixImage{packageName, module, sources}
}

type nixImage struct {
	packageName schema.PackageName
	module      *workspace.Module
	sources     fs.FS
}

func (l nixImage) BuildImage(ctx context.Context, env planning.Context, conf build.Configuration) (compute.Computable[oci.Image], error) {
	if conf.TargetPlatform() == nil {
		return nil, fnerrors.BadInputError("nix: target platform is missing")
	}

	const target = "/source"
	const outputImageFile = "image.tgz"

	var err error
	sourceFiles, err := llbutil.WriteFS(ctx, l.sources, llb.Scratch(), "/")
	if err != nil {
		return nil, err
	}

	nixosImage, err := pins.CheckImage("nixos/nix:2.6.0")
	if err != nil {
		return nil, err
	}

	const nixconf = `# Namespace-managed nix configuration.
build-users-group = nixbld
sandbox = false
trusted-public-keys = cache.nixos.org-1:6NCHdD59X431o0gWypbMrAURkbJ16ZPMQFGspcDShjY=
experimental-features = nix-command flakes
filter-syscalls = false
	`

	// Filter-syscalls is necessary due to an interaction with Docker, see https://github.com/NixOS/nix/issues/5258

	base := llb.Image(nixosImage, llb.Platform(*conf.TargetPlatform())).
		File(llb.Mkfile("/etc/nix/nix.conf", 0777, []byte(nixconf))).
		AddEnv("PATH", "/root/.nix-profile/bin")

	build := base.
		Run(llb.Shlexf("nix build %s -o /tmp/result", target))
	build.AddMount("/source", sourceFiles)

	// nix build produces a symlink to the result, which we then need to copy into the target mount so buildkit copies it out.
	postCopy := build.Root().
		Run(llb.Shlexf("cp -L /tmp/result /out/" + outputImageFile))
	out := postCopy.AddMount("/out", llb.Scratch())

	fsys, err := buildkit.LLBToFS(ctx, env, conf, out)
	if err != nil {
		return nil, err
	}

	return compute.Transform(fsys, func(ctx context.Context, fsys fs.FS) (oci.Image, error) {
		return buildkit.IngestFromFS(ctx, fsys, outputImageFile, true)
	}), nil
}

func (l nixImage) PlatformIndependent() bool { return false }

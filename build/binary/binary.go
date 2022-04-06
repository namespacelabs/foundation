// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package binary

import (
	"context"
	"fmt"
	"io/fs"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/build/buildkit"
	"namespacelabs.dev/foundation/build/multiplatform"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/compute"
)

var UsePrebuilts = true // XXX make these a scoped configuration instead.

var BuildGo func(loc workspace.Location, goPackage, binName string, unsafeCacheable bool) (build.Spec, error)
var BuildWeb func(workspace.Location) build.Spec
var BuildLLBGen func(schema.PackageName, *workspace.Module, build.Spec) build.Spec
var BuildNix func(schema.PackageName, *workspace.Module, fs.FS) build.Spec

const LLBGenBinaryName = "llbgen"

type Prepared struct {
	Name    string
	Plan    build.Plan
	Command []string
}

type PreparedImage struct {
	Name    string
	Image   compute.Computable[oci.Image]
	Command []string
}

type BuildImageOpts struct {
	UsePrebuilts bool
	Platforms    []specs.Platform
}

func ValidateIsBinary(pkg *workspace.Package) error {
	if pkg.Binary == nil {
		return fnerrors.UserError(pkg.Location, "expected a binary")
	}

	return nil
}

// Returns a Prepared.
func Plan(ctx context.Context, pkg *workspace.Package, opts BuildImageOpts) (*Prepared, error) {
	if err := ValidateIsBinary(pkg); err != nil {
		return nil, err
	}

	loc := pkg.Location

	spec, err := planImage(ctx, loc, pkg.Binary, opts)
	if err != nil {
		return nil, err
	}

	platforms := opts.Platforms
	if spec.PlatformIndependent() {
		platforms = nil
	}

	plan := build.Plan{
		SourcePackage: loc.PackageName,
		SourceLabel:   fmt.Sprintf("Binary %s", loc.PackageName),
		Spec:          spec,
		Workspace:     loc.Module,
		Platforms:     platforms,
	}

	return &Prepared{
		Name:    loc.PackageName.String(),
		Plan:    plan,
		Command: Command(pkg),
		// XXX pass args, and env.
	}, nil
}

func Command(pkg *workspace.Package) []string {
	return pkg.Binary.GetConfig().GetCommand()
}

func (p Prepared) Image(ctx context.Context, env ops.Environment) (compute.Computable[oci.ResolvableImage], error) {
	return multiplatform.PrepareMultiPlatformImage(ctx, env, p.Plan)
}

func PlanImage(ctx context.Context, pkg *workspace.Package, env ops.Environment, usePrebuilts bool, platform *specs.Platform) (*PreparedImage, error) {
	if pkg.Binary == nil {
		return nil, fnerrors.UserError(pkg.Location, "expected a binary")
	}

	loc := pkg.Location

	spec, err := planImage(ctx, loc, pkg.Binary, BuildImageOpts{UsePrebuilts: usePrebuilts})
	if err != nil {
		return nil, err
	}

	img, err := spec.BuildImage(ctx, env, build.Configuration{
		SourceLabel: fmt.Sprintf("Binary %s", loc.PackageName),
		Workspace:   loc.Module,
		Target:      platform,
	})
	if err != nil {
		return nil, err
	}

	return &PreparedImage{
		Name:    loc.PackageName.String(),
		Image:   img,
		Command: Command(pkg),
		// XXX pass args, and env.
	}, nil
}

func planImage(ctx context.Context, loc workspace.Location, bin *schema.Binary, opts BuildImageOpts) (build.Spec, error) {
	if bin.From == nil {
		return nil, fnerrors.UserError(loc, "don't know how to build binary image: `from` statement is missing")
	}

	// We prepare the build spec, as we need information, e.g. whether it's platform independent,
	// if a prebuilt is specified.
	spec, err := buildSpec(ctx, loc, bin)
	if err != nil {
		return nil, err
	}

	if opts.UsePrebuilts && UsePrebuilts {

		for _, prebuilt := range loc.Module.Workspace.PrebuiltBinary {
			if prebuilt.PackageName == loc.PackageName.String() {
				imgid := oci.ImageID{Repository: prebuilt.Repository, Digest: prebuilt.Digest}
				return build.PrebuiltPlan(imgid, spec.PlatformIndependent()), nil
			}
		}
	}

	return spec, nil
}

func buildSpec(ctx context.Context, loc workspace.Location, bin *schema.Binary) (build.Spec, error) {
	src := bin.From

	if goPackage := src.GoPackage; goPackage != "" {
		// Note, regardless of what config.command has been set to, we always build a
		// binary named bin.Name.
		return BuildGo(loc, goPackage, bin.Name, false)
	}

	if wb := src.WebBuild; wb != "" {
		if wb != "." {
			return nil, fnerrors.UserError(loc, "web_build: must be set to `.`")
		}
		return BuildWeb(loc), nil
	}

	if llb := src.LlbGoBinary; llb != "" {
		// We allow these Go binaries to be cached because we expect them to be seldom
		// changed, and the impact of not being able to verify the cache is too big.
		spec, err := BuildGo(loc, llb, LLBGenBinaryName, true)
		if err != nil {
			return nil, err
		}
		return BuildLLBGen(loc.PackageName, loc.Module, spec), nil
	}

	if nix := src.NixFlake; nix != "" {
		fsys, err := loc.Module.SnapshotContents(ctx, loc.Rel(nix))
		if err != nil {
			return nil, fnerrors.Wrap(loc, err)
		}
		return BuildNix(loc.PackageName, loc.Module, fsys), nil
	}

	if dockerFile := src.Dockerfile; dockerFile != "" {
		fsys, err := loc.Module.SnapshotContents(ctx, loc.Rel())
		if err != nil {
			return nil, fnerrors.Wrap(loc, err)
		}

		contents, err := fs.ReadFile(fsys, dockerFile)
		if err != nil {
			return nil, fnerrors.Wrapf(loc, err, "failed to load Dockerfile")
		}

		// XXX consistency: we've already loaded the workspace contents, ideally we'd use those.
		spec, err := buildkit.DockerfileBuild(buildkit.LocalContents{
			Module: loc.Module, Path: loc.Rel(),
		}, contents)
		if err != nil {
			return nil, fnerrors.Wrap(loc, err)
		}

		return spec, nil
	}

	return nil, fnerrors.UserError(loc, "don't know how to build binary image: `from` statement does not yield a build unit")
}

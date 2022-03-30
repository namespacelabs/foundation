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
	"namespacelabs.dev/foundation/internal/artifacts/registry"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/compute"
)

var UsePrebuilts = true // XXX make these a scoped configuration instead.

var BuildGo func(loc workspace.Location, goPackage, binName string, unsafeCacheable bool) (build.Spec, error)
var BuildWeb func(workspace.Location) build.Spec

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

// Returns a Prepared.
func Plan(ctx context.Context, pkg *workspace.Package, opts BuildImageOpts) (*Prepared, error) {
	if pkg.Binary == nil {
		return nil, fnerrors.UserError(pkg.Location, "expected a binary")
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
		Command: []string{Command(pkg)},
	}, nil
}

func Command(pkg *workspace.Package) string {
	if pkg.Binary == nil {
		return ""
	}
	return "/" + pkg.Binary.Name
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
		Command: []string{"/" + pkg.Binary.Name},
	}, nil
}

func MakeTag(ctx context.Context, env ops.Environment, pkg *workspace.Package, bid provision.BuildID, keepRepos bool) (compute.Computable[oci.AllocatedName], error) {
	if pkg.Binary == nil {
		return nil, fnerrors.UserError(pkg.Location, "expected a binary")
	}

	if keepRepos {
		// Build a new tag that looks like the original tag, but with the current build ID.
		// Creating a tag is required because unfortunately uploading an image to Docker
		// requires a tag, even though we operate on digests.
		return registry.StaticName(nil, oci.ImageID{
			Repository: pkg.Binary.Repository,
			Tag:        bid.String(),
		}), nil
	}

	return registry.AllocateName(ctx, env, pkg.Location.PackageName, bid)
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
		if bin.Digest != "" {
			imgid := oci.ImageID{Repository: bin.Repository, Digest: bin.Digest}
			return build.PrebuiltPlan(imgid, spec.PlatformIndependent()), nil
		}

		for _, prebuilt := range loc.Module.Workspace.PrebuiltBinary {
			if prebuilt.PackageName == loc.PackageName.String() {
				imgid := oci.ImageID{Repository: bin.Repository, Digest: prebuilt.Digest}
				return build.PrebuiltPlan(imgid, spec.PlatformIndependent()), nil
			}
		}
	}

	return spec, nil
}

func buildSpec(ctx context.Context, loc workspace.Location, bin *schema.Binary) (build.Spec, error) {
	src := bin.From

	if goPackage := src.GoPackage; goPackage != "" {
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
		spec, err := BuildGo(loc, llb, llbGenBinaryName, true)
		if err != nil {
			return nil, err
		}
		return llbBinary{loc.PackageName, loc.Module, spec}, nil
	}

	if nix := src.NixFlake; nix != "" {
		fsys, err := loc.Module.SnapshotContents(ctx, loc.Rel(nix))
		if err != nil {
			return nil, fnerrors.Wrap(loc, err)
		}

		return nixImage{loc.PackageName, loc.Module, fsys}, nil
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

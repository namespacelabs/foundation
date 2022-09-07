// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package binary

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/build/buildkit"
	"namespacelabs.dev/foundation/build/multiplatform"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/artifacts/registry"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/storage"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace/compute"
)

var (
	UsePrebuilts  = true // XXX make these a scoped configuration instead.
	PrebuiltsFlag = ""
)

var BuildGo func(loc pkggraph.Location, _ *schema.ImageBuildPlan_GoBuild, unsafeCacheable bool) (build.Spec, error)
var BuildWeb func(pkggraph.Location) build.Spec
var BuildLLBGen func(schema.PackageName, *pkggraph.Module, build.Spec) build.Spec
var BuildNix func(schema.PackageName, *pkggraph.Module, fs.FS) build.Spec

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

func GetBinary(pkg *pkggraph.Package, binName string) (*schema.Binary, error) {
	for _, bin := range pkg.Binaries {
		if bin.Name == binName {
			return bin, nil
		}
	}

	if binName == "" && len(pkg.Binaries) == 1 {
		return pkg.Binaries[0], nil
	}

	return nil, fnerrors.UserError(pkg.Location, "no such binary %q", binName)
}

// Returns a Prepared.
func Plan(ctx context.Context, pkg *pkggraph.Package, binName string, opts BuildImageOpts) (*Prepared, error) {
	binary, err := GetBinary(pkg, binName)
	if err != nil {
		return nil, err
	}

	loc := pkg.Location

	spec, err := planImage(ctx, loc, binary, opts)
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
		BuildKind:     storage.Build_BINARY,
		Spec:          spec,
		Workspace:     loc.Module,
		Platforms:     platforms,
	}

	return &Prepared{
		Name:    loc.PackageName.String(),
		Plan:    plan,
		Command: binary.Config.GetCommand(),
		// XXX pass args, and env.
	}, nil
}

func (p Prepared) Image(ctx context.Context, env planning.Context) (compute.Computable[oci.ResolvableImage], error) {
	return multiplatform.PrepareMultiPlatformImage(ctx, env, p.Plan)
}

func PlanImage(ctx context.Context, pkg *pkggraph.Package, binName string, env planning.Context, usePrebuilts bool, platform *specs.Platform) (*PreparedImage, error) {
	binary, err := GetBinary(pkg, binName)
	if err != nil {
		return nil, err
	}

	loc := pkg.Location

	spec, err := planImage(ctx, loc, binary, BuildImageOpts{UsePrebuilts: usePrebuilts})
	if err != nil {
		return nil, err
	}

	img, err := spec.BuildImage(ctx, env, build.NewBuildTarget(platform).
		WithSourcePackage(loc.PackageName).
		WithSourceLabel("Binary %s", loc.PackageName).
		WithWorkspace(loc.Module))
	if err != nil {
		return nil, err
	}

	return &PreparedImage{
		Name:    loc.PackageName.String(),
		Image:   img,
		Command: binary.Config.GetCommand(),
		// XXX pass args, and env.
	}, nil
}

func PrebuiltImageID(loc pkggraph.Location) (*oci.ImageID, error) {
	if !UsePrebuilts {
		return nil, nil
	}

	var fromFlag, fromWorkspace *oci.ImageID
	for _, overwrite := range strings.Split(PrebuiltsFlag, ",") {
		parts := strings.SplitN(overwrite, ":", 2)
		if len(parts) != 2 {
			break // Silently fail.
		}
		if parts[0] != loc.PackageName.String() {
			continue
		}

		parts = strings.SplitN(parts[1], "@", 2)
		if len(parts) != 2 {
			break // Silently fail.
		}

		fromFlag = &oci.ImageID{Repository: parts[0], Digest: parts[1]}
	}

	for _, prebuilt := range loc.Module.Workspace.PrebuiltBinary {
		if prebuilt.PackageName == loc.PackageName.String() {
			imgid := oci.ImageID{Repository: prebuilt.Repository, Digest: prebuilt.Digest}
			if imgid.Repository == "" {
				if loc.Module.Workspace.PrebuiltBaseRepository == "" {
					break // Silently fail.
				}
				imgid.Repository = filepath.Join(loc.Module.Workspace.PrebuiltBaseRepository, prebuilt.PackageName)
			}

			fromWorkspace = &imgid
		}
	}

	if fromFlag != nil && fromWorkspace != nil {
		if fromFlag.Repository != fromWorkspace.Repository {
			return nil, fnerrors.UserError(loc, "conflicting repositories for prebuilt: %s vs %s", fromFlag.Repository, fromWorkspace.Repository)
		}
		if fromFlag.Digest != fromWorkspace.Digest {
			return nil, fnerrors.UserError(loc, "conflicting digest for prebuilt: %s vs %s", fromFlag.Digest, fromWorkspace.Digest)
		}
	}

	if fromFlag != nil {
		return fromFlag, nil
	}

	return fromWorkspace, nil
}

func planImage(ctx context.Context, loc pkggraph.Location, bin *schema.Binary, opts BuildImageOpts) (build.Spec, error) {
	// We prepare the build spec, as we need information, e.g. whether it's platform independent,
	// if a prebuilt is specified.
	spec, err := buildLayeredSpec(ctx, loc, bin)
	if err != nil {
		return nil, err
	}

	if opts.UsePrebuilts {
		imgid, err := PrebuiltImageID(loc)
		if err != nil {
			return nil, err
		}
		if imgid != nil {
			public := true // We assume all prebuilts are public, unless noted otherwise.
			return build.PrebuiltPlan(*imgid, spec.PlatformIndependent(), oci.ResolveOpts{PublicImage: public}), nil
		}
	}

	return spec, nil
}

func buildLayeredSpec(ctx context.Context, loc pkggraph.Location, bin *schema.Binary) (build.Spec, error) {
	src := bin.BuildPlan

	if src == nil || len(src.LayerBuildPlan) == 0 {
		return nil, fnerrors.UserError(loc, "%s: don't know how to build, missing build plan", bin.Name)
	}

	if len(src.LayerBuildPlan) == 1 {
		return buildSpec(ctx, loc, bin, src.LayerBuildPlan[0])
	}

	specs := make([]build.Spec, len(src.LayerBuildPlan))
	platformIndependent := true
	for k, plan := range src.LayerBuildPlan {
		var err error
		specs[k], err = buildSpec(ctx, loc, bin, plan)
		if err != nil {
			return nil, err
		}
		if !specs[k].PlatformIndependent() {
			platformIndependent = false
		}
	}

	return mergeSpecs{specs, platformIndependent}, nil
}

func buildSpec(ctx context.Context, loc pkggraph.Location, bin *schema.Binary, src *schema.ImageBuildPlan) (build.Spec, error) {
	if src == nil {
		return nil, fnerrors.UserError(loc, "don't know how to build %q: no plan", bin.Name)
	}

	if goPackage := src.GoPackage; goPackage != "" {
		// Note, regardless of what config.command has been set to, we always build a
		// binary named bin.Name.
		return BuildGo(loc, &schema.ImageBuildPlan_GoBuild{
			RelPath:    goPackage,
			BinaryName: bin.Name,
		}, false)
	}

	if src.GoBuild != nil {
		return BuildGo(loc, src.GoBuild, false)
	}

	if wb := src.WebBuild; wb != "" {
		if wb != "." {
			return nil, fnerrors.UserError(loc, "web_build: must be set to `.`")
		}
		return BuildWeb(loc), nil
	}

	if llb := src.LlbPlan; llb != nil {
		spec, err := buildLayeredSpec(ctx, loc, llb.OutputOf)
		if err != nil {
			return nil, err
		}

		return BuildLLBGen(loc.PackageName, loc.Module, spec), nil
	}

	if nix := src.NixFlake; nix != "" {
		fsys, err := compute.GetValue(ctx, loc.Module.VersionedFS(loc.Rel(nix), false))
		if err != nil {
			return nil, fnerrors.Wrap(loc, err)
		}
		return BuildNix(loc.PackageName, loc.Module, fsys.FS()), nil
	}

	if dockerFile := src.Dockerfile; dockerFile != "" {
		fsys, err := compute.GetValue(ctx, loc.Module.VersionedFS(loc.Rel(), false))
		if err != nil {
			return nil, fnerrors.Wrap(loc, err)
		}

		contents, err := fs.ReadFile(fsys.FS(), dockerFile)
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

	if len(src.SnapshotFiles) > 0 {
		return snapshotFiles{loc.Rel(), src.SnapshotFiles}, nil
	}

	return nil, fnerrors.UserError(loc, "don't know how to build binary image: `from` statement does not yield a build unit")
}

func EnsureImage(ctx context.Context, env planning.Context, prepared *Prepared) (oci.ImageID, error) {
	img, err := prepared.Image(ctx, env)
	if err != nil {
		return oci.ImageID{}, err
	}

	name, err := registry.RawAllocateName(ctx, env.Configuration(), prepared.Name)
	if err != nil {
		return oci.ImageID{}, err
	}

	return compute.GetValue(ctx, oci.PublishResolvable(name, img))
}

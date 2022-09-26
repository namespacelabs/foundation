// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package binary

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/build/buildkit"
	"namespacelabs.dev/foundation/build/multiplatform"
	"namespacelabs.dev/foundation/engine/compute"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/artifacts/registry"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/storage"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/planning"
)

var (
	UsePrebuilts = true // XXX make these a scoped configuration instead.
)

var BuildGo func(loc pkggraph.Location, _ *schema.ImageBuildPlan_GoBuild, unsafeCacheable bool) (build.Spec, error)
var BuildWeb func(pkggraph.Location) build.Spec
var BuildLLBGen func(schema.PackageName, *pkggraph.Module, build.Spec) build.Spec
var BuildNix func(schema.PackageName, *pkggraph.Module, fs.FS) build.Spec
var BuildNodejs func(planning.Context, pkggraph.Location, *schema.ImageBuildPlan_NodejsBuild, bool /* isFocus */) (build.Spec, error)

var prebuiltsConfType = planning.DefineConfigType[*Prebuilts]()

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
	// Whether the server build is triggered explicitly, e.g. as a parameter of `ns dev` and not as a dependant server.
	// Builders may use this flag to listen to the FS changes and do hot restart, for example.
	IsFocus   bool
	Platforms []specs.Platform
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
func Plan(ctx context.Context, pkg *pkggraph.Package, binName string, env pkggraph.SealedContext, opts BuildImageOpts) (*Prepared, error) {
	binary, err := GetBinary(pkg, binName)
	if err != nil {
		return nil, err
	}

	return PlanBinary(ctx, pkg.Location, binary, env, opts)
}

func PlanBinary(ctx context.Context, loc pkggraph.Location, binary *schema.Binary, env pkggraph.SealedContext, opts BuildImageOpts) (*Prepared, error) {
	spec, err := planImage(ctx, loc, binary, env, opts)
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

func (p Prepared) Image(ctx context.Context, env pkggraph.SealedContext) (compute.Computable[oci.ResolvableImage], error) {
	return multiplatform.PrepareMultiPlatformImage(ctx, env, p.Plan)
}

func PlanImage(ctx context.Context, pkg *pkggraph.Package, binName string, env pkggraph.SealedContext, usePrebuilts bool, platform *specs.Platform) (*PreparedImage, error) {
	binary, err := GetBinary(pkg, binName)
	if err != nil {
		return nil, err
	}

	loc := pkg.Location

	spec, err := planImage(ctx, loc, binary, env, BuildImageOpts{UsePrebuilts: usePrebuilts})
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

func PrebuiltImageID(ctx context.Context, loc pkggraph.Location, env planning.Context) (*oci.ImageID, error) {
	if !UsePrebuilts {
		return nil, nil
	}

	prebuilts := loc.Module.Workspace.PrebuiltBinary

	if conf, ok := prebuiltsConfType.CheckGet(env.Configuration()); ok {
		prebuilts = append(prebuilts, conf.PrebuiltBinary...)
		fmt.Fprintf(console.Debug(ctx), "Adding %d prebuilts from planning configuration.", len(conf.PrebuiltBinary))
	}

	var selected *oci.ImageID
	for _, prebuilt := range prebuilts {
		if prebuilt.PackageName == loc.PackageName.String() {
			imgid := oci.ImageID{Repository: prebuilt.Repository, Digest: prebuilt.Digest}
			if imgid.Repository == "" {
				if loc.Module.Workspace.PrebuiltBaseRepository == "" {
					break // Silently fail.
				}
				imgid.Repository = filepath.Join(loc.Module.Workspace.PrebuiltBaseRepository, prebuilt.PackageName)
			}

			if selected == nil {
				selected = &imgid
				continue
			}

			if imgid.Repository != selected.Repository {
				return nil, fnerrors.UserError(loc, "conflicting repositories for prebuilt: %s vs %s", imgid.Repository, selected.Repository)
			}
			if imgid.Digest != selected.Digest {
				return nil, fnerrors.UserError(loc, "conflicting digest for prebuilt: %s vs %s", imgid.Digest, selected.Digest)
			}
		}
	}

	return selected, nil
}

func planImage(ctx context.Context, loc pkggraph.Location, bin *schema.Binary, env pkggraph.SealedContext, opts BuildImageOpts) (build.Spec, error) {
	// We prepare the build spec, as we need information, e.g. whether it's platform independent,
	// if a prebuilt is specified.
	spec, err := buildLayeredSpec(ctx, loc, bin, env, opts)
	if err != nil {
		return nil, err
	}

	if opts.UsePrebuilts {
		imgid, err := PrebuiltImageID(ctx, loc, env)
		if err != nil {
			return nil, err
		}
		if imgid != nil {
			return build.PrebuiltPlan(*imgid, spec.PlatformIndependent(), build.PrebuiltResolveOpts()), nil
		}
	}

	return spec, nil
}

func buildLayeredSpec(ctx context.Context, loc pkggraph.Location, bin *schema.Binary, env pkggraph.SealedContext, opts BuildImageOpts) (build.Spec, error) {
	src := bin.BuildPlan

	if src == nil || len(src.LayerBuildPlan) == 0 {
		return nil, fnerrors.UserError(loc, "%s: don't know how to build, missing build plan", bin.Name)
	}

	if len(src.LayerBuildPlan) == 1 {
		return buildSpec(ctx, loc, bin, src.LayerBuildPlan[0], env, opts)
	}

	specs := make([]build.Spec, len(src.LayerBuildPlan))
	platformIndependent := true
	for k, plan := range src.LayerBuildPlan {
		var err error
		specs[k], err = buildSpec(ctx, loc, bin, plan, env, opts)
		if err != nil {
			return nil, err
		}
		if !specs[k].PlatformIndependent() {
			platformIndependent = false
		}
	}

	return mergeSpecs{specs, platformIndependent}, nil
}

func buildSpec(ctx context.Context, loc pkggraph.Location, bin *schema.Binary, src *schema.ImageBuildPlan, env pkggraph.SealedContext, opts BuildImageOpts) (build.Spec, error) {
	if src == nil {
		return nil, fnerrors.UserError(loc, "don't know how to build %q: no plan", bin.Name)
	}

	if imageId := src.ImageId; imageId != "" {
		imgId, err := oci.ParseImageID(imageId)
		if err != nil {
			return nil, err
		}

		return build.PrebuiltPlan(imgId, false /* platformIndependent */, build.PrebuiltResolveOpts()), nil
	}

	if goPackage := src.GoPackage; goPackage != "" {
		// Note, regardless of what config.command has been set to, we always build a
		// binary named bin.Name.
		return BuildGo(loc, &schema.ImageBuildPlan_GoBuild{
			RelPath:    goPackage,
			BinaryName: bin.Name,
			IsFocus:    opts.IsFocus,
		}, false)
	}

	if src.GoBuild != nil {
		return BuildGo(loc, src.GoBuild, false)
	}

	if src.NodejsBuild != nil {
		return BuildNodejs(env, loc, src.NodejsBuild, opts.IsFocus)
	}

	if wb := src.WebBuild; wb != "" {
		if wb != "." {
			return nil, fnerrors.UserError(loc, "web_build: must be set to `.`")
		}
		return BuildWeb(loc), nil
	}

	if llb := src.LlbPlan; llb != nil {
		spec, err := buildLayeredSpec(ctx, loc, llb.OutputOf, env, opts)
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
		spec, err := buildkit.DockerfileBuild(loc.Rel(), dockerFile, opts.IsFocus)
		if err != nil {
			return nil, fnerrors.Wrap(loc, err)
		}

		return spec, nil
	}

	if binRef := src.Binary; binRef != nil {
		binPkg, err := env.LoadByName(ctx, binRef.AsPackageName())
		if err != nil {
			return nil, err
		}

		binary, err := GetBinary(binPkg, binRef.Name)
		if err != nil {
			return nil, err
		}

		spec, err := planImage(ctx, binPkg.Location, binary, env, opts)
		if err != nil {
			return nil, err
		}

		return spec, nil
	}

	if len(src.SnapshotFiles) > 0 {
		return snapshotFiles{loc.Rel(), src.SnapshotFiles}, nil
	}

	return nil, fnerrors.UserError(loc, "don't know how to build binary image: `from` statement does not yield a build unit")
}

func EnsureImage(ctx context.Context, env pkggraph.SealedContext, prepared *Prepared) (oci.ImageID, error) {
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

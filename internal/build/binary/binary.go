// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package binary

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/artifacts/registry"
	"namespacelabs.dev/foundation/internal/build"
	"namespacelabs.dev/foundation/internal/build/assets"
	"namespacelabs.dev/foundation/internal/build/multiplatform"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/integrations/dockerfile"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/storage"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/pkggraph"
)

var (
	UsePrebuilts = true // XXX make these a scoped configuration instead.
)

var BuildGo func(loc pkggraph.Location, _ *schema.ImageBuildPlan_GoBuild, unsafeCacheable bool) (build.Spec, error)
var BuildLLBGen func(schema.PackageName, *pkggraph.Module, build.Spec) build.Spec
var BuildAlpine func(pkggraph.Location, *schema.ImageBuildPlan_AlpineBuild) build.Spec
var BuildNix func(schema.PackageName, *pkggraph.Module, fs.FS) build.Spec
var BuildNodejs func(cfg.Context, pkggraph.Location, *schema.NodejsBuild, assets.AvailableBuildAssets) (build.Spec, error)
var BuildStaticFilesServer func(*schema.ImageBuildPlan_StaticFilesServer) build.Spec

var prebuiltsConfType = cfg.DefineConfigType[*Prebuilts]()

const LLBGenBinaryName = "llbgen"

type Prepared struct {
	Name       string
	Plan       build.Plan
	Command    []string
	Args       []string
	Env        []*schema.BinaryConfig_EnvEntry
	WorkingDir string
	Labels     []*schema.Label
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
func Plan(ctx context.Context, pkg *pkggraph.Package, binName string, env pkggraph.SealedContext, assets assets.AvailableBuildAssets, opts BuildImageOpts) (*Prepared, error) {
	binary, err := pkg.LookupBinary(binName)
	if err != nil {
		return nil, err
	}

	return PlanBinary(ctx, env, env, pkg.Location, binary, assets, opts)
}

func PlanBinary(ctx context.Context, pl pkggraph.PackageLoader, env cfg.Context, loc pkggraph.Location, binary *schema.Binary, assets assets.AvailableBuildAssets, opts BuildImageOpts) (*Prepared, error) {
	spec, err := planImage(ctx, pl, env, loc, binary, assets, opts)
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
		Name:       loc.PackageName.String(),
		Plan:       plan,
		Command:    binary.Config.GetCommand(),
		Args:       binary.Config.GetArgs(),
		Env:        binary.Config.GetEnv(),
		WorkingDir: binary.Config.GetWorkingDir(),
		Labels:     binary.GetLabels(),
	}, nil
}

func (p Prepared) Image(ctx context.Context, env pkggraph.SealedContext) (compute.Computable[oci.ResolvableImage], error) {
	return multiplatform.PrepareMultiPlatformImage(ctx, env, p.Plan)
}

func PrebuiltImageID(ctx context.Context, loc pkggraph.Location, cfg cfg.Configuration) (*oci.ImageID, error) {
	if !UsePrebuilts {
		return nil, nil
	}

	prebuilts := loc.Module.Workspace.PrebuiltBinary

	if conf, ok := prebuiltsConfType.CheckGet(cfg); ok {
		prebuilts = append(prebuilts, conf.PrebuiltBinary...)
		fmt.Fprintf(console.Debug(ctx), "Adding %d prebuilts from planning configuration.\n", len(conf.PrebuiltBinary))
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
				return nil, fnerrors.NewWithLocation(loc, "conflicting repositories for prebuilt: %s vs %s", imgid.Repository, selected.Repository)
			}
			if imgid.Digest != selected.Digest {
				return nil, fnerrors.NewWithLocation(loc, "conflicting digest for prebuilt: %s vs %s", imgid.Digest, selected.Digest)
			}
		}
	}

	return selected, nil
}

func planImage(ctx context.Context, pl pkggraph.PackageLoader, env cfg.Context, loc pkggraph.Location, bin *schema.Binary, assets assets.AvailableBuildAssets, opts BuildImageOpts) (build.Spec, error) {
	// We prepare the build spec, as we need information, e.g. whether it's platform independent,
	// if a prebuilt is specified.
	spec, err := buildLayeredSpec(ctx, pl, env, loc, bin, assets, opts)
	if err != nil {
		return nil, err
	}

	if opts.UsePrebuilts {
		imgid, err := PrebuiltImageID(ctx, loc, env.Configuration())
		if err != nil {
			return nil, err
		}

		if imgid != nil {
			return build.PrebuiltPlan(*imgid, spec.PlatformIndependent(), build.PrebuiltResolveOpts()), nil
		}
	}

	return spec, nil
}

func buildLayeredSpec(ctx context.Context, pl pkggraph.PackageLoader, env cfg.Context, loc pkggraph.Location, bin *schema.Binary, assets assets.AvailableBuildAssets, opts BuildImageOpts) (build.Spec, error) {
	src := bin.BuildPlan

	if src == nil || len(src.LayerBuildPlan) == 0 {
		return nil, fnerrors.NewWithLocation(loc, "%s: don't know how to build, missing build plan", bin.Name)
	}

	if len(src.LayerBuildPlan) == 1 {
		return buildSpec(ctx, pl, env, loc, bin, src.LayerBuildPlan[0], assets, opts)
	}

	specs := make([]build.Spec, len(src.LayerBuildPlan))
	descriptions := make([]string, len(src.LayerBuildPlan))
	platformIndependent := true
	for k, plan := range src.LayerBuildPlan {
		var err error
		specs[k], err = buildSpec(ctx, pl, env, loc, bin, plan, assets, opts)
		if err != nil {
			return nil, err
		}

		if !specs[k].PlatformIndependent() {
			platformIndependent = false
		}

		if plan.Description == "" {
			descriptions[k] = fmt.Sprintf("plan#%d", k)
		} else {
			descriptions[k] = plan.Description
		}
	}

	return MergeSpecs{Specs: specs, Descriptions: descriptions, platformIndependent: platformIndependent}, nil
}

func buildSpec(ctx context.Context, pl pkggraph.PackageLoader, env cfg.Context, loc pkggraph.Location, bin *schema.Binary, src *schema.ImageBuildPlan, assets assets.AvailableBuildAssets, opts BuildImageOpts) (build.Spec, error) {
	if src == nil {
		return nil, fnerrors.NewWithLocation(loc, "don't know how to build %q: no plan", bin.Name)
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
		}, false)
	}

	if src.GoBuild != nil {
		return BuildGo(loc, src.GoBuild, false)
	}

	if src.NodejsBuild != nil {
		return BuildNodejs(env, loc, src.NodejsBuild, assets)
	}

	if llb := src.LlbPlan; llb != nil {
		spec, err := buildLayeredSpec(ctx, pl, env, loc, llb.OutputOf, assets, opts)
		if err != nil {
			return nil, err
		}

		return BuildLLBGen(loc.PackageName, loc.Module, spec), nil
	}

	if alpine := src.AlpineBuild; alpine != nil {
		return BuildAlpine(loc, alpine), nil
	}

	if nix := src.NixFlake; nix != "" {
		return BuildNix(loc.PackageName, loc.Module, loc.Module.ReadOnlyFS()), nil
	}

	if dockerFile := src.Dockerfile; dockerFile != "" {
		spec, err := dockerfile.Build(loc.Rel(), dockerFile)
		if err != nil {
			return nil, fnerrors.AttachLocation(loc, err)
		}

		return spec, nil
	}

	if binRef := src.Binary; binRef != nil {
		binPkg, binary, err := pkggraph.LoadBinary(ctx, pl, binRef)
		if err != nil {
			return nil, err
		}

		spec, err := planImage(ctx, pl, env, binPkg.Location, binary, assets, opts)
		if err != nil {
			return nil, err
		}

		return spec, nil
	}

	if src.StaticFilesServer != nil {
		return BuildStaticFilesServer(src.StaticFilesServer), nil
	}

	if len(src.SnapshotFiles) > 0 {
		return snapshotFiles{loc.Rel(), src.SnapshotFiles}, nil
	}

	if src.FilesFrom != nil {
		inner, err := buildSpec(ctx, pl, env, loc, bin, src.FilesFrom.From, assets, opts)
		if err != nil {
			return nil, err
		}

		return filesFrom{inner, src.FilesFrom.Files, src.FilesFrom.TargetDir}, nil
	}

	if src.MakeSquashfs != nil {
		inner, err := buildSpec(ctx, pl, env, loc, bin, src.MakeSquashfs.From, assets, opts)
		if err != nil {
			return nil, err
		}

		return makeSquashFS{inner, src.MakeSquashfs.Target}, nil
	}

	if src.MakeFsImage != nil {
		inner, err := buildSpec(ctx, pl, env, loc, bin, src.MakeFsImage.From, assets, opts)
		if err != nil {
			return nil, err
		}

		switch strings.ToLower(src.MakeFsImage.Kind) {
		case "squashfs", "squash":
			return makeSquashFS{inner, src.MakeFsImage.Target}, nil

		case "ext4fs", "ext4":
			return makeExt4Image{inner, src.MakeFsImage.Target, src.MakeFsImage.Size}, nil

		default:
			return nil, fnerrors.BadInputError("make_fs_image: unsupported filesystem %q", src.MakeFsImage.Kind)
		}
	}

	return nil, fnerrors.NewWithLocation(loc, "don't know how to build binary image: `from` statement does not yield a build unit")
}

func EnsureImage(ctx context.Context, env pkggraph.SealedContext, registry registry.Manager, prepared *Prepared) (oci.ImageID, error) {
	img, err := prepared.Image(ctx, env)
	if err != nil {
		return oci.ImageID{}, err
	}

	name := registry.AllocateName(prepared.Name)

	return compute.GetValue(ctx, oci.PublishResolvable(name, img, prepared.Plan))
}

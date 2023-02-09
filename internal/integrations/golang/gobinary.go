// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package golang

import (
	"context"
	"path/filepath"

	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/build"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/gosupport"
	"namespacelabs.dev/foundation/internal/parsing/invariants"
	"namespacelabs.dev/foundation/internal/versions"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg/knobs"
	"namespacelabs.dev/foundation/std/pkggraph"
)

const Version_GolangBaseVersionMarker = 68

type GoBinary struct {
	PackageName schema.PackageName `json:"packageName"`

	GoModulePath string `json:"modulePath"` // Relative to workspace root.
	GoModule     string `json:"module"`     // Go module name.
	GoVersion    string `json:"goVersion"`
	SourcePath   string `json:"sourcePath"` // Relative to workspace root.
	BinaryName   string `json:"binaryName"`

	BinaryOnly      bool
	UnsafeCacheable bool // Unsafe because we can't guarantee that the sources used for compilation are consistent with the workspace contents.
}

var UseBuildKitForBuilding = knobs.Bool("golang_use_buildkit", "If set to true, buildkit is used for building, instead of a ko-style builder.", false)

func (gb GoBinary) BuildImage(ctx context.Context, env pkggraph.SealedContext, conf build.Configuration) (compute.Computable[oci.Image], error) {
	if conf.PrefersBuildkit() || UseBuildKitForBuilding.Get(env.Configuration()) {
		return buildUsingBuildkit(ctx, env, gb, conf)
	}

	if conf.Workspace() == nil {
		panic(conf)
	}

	return buildLocalImage(ctx, env, conf.Workspace(), gb, conf)
}

func (gb GoBinary) PlatformIndependent() bool { return false }

func FromLocation(loc pkggraph.Location, pkgName string) (*GoBinary, error) {
	absSrc := loc.Abs(pkgName)
	mod, modFile, err := gosupport.LookupGoModule(absSrc)
	if err != nil {
		return nil, err
	}

	relMod, err := filepath.Rel(loc.Module.Abs(), modFile)
	if err != nil {
		return nil, err
	}

	return &GoBinary{
		PackageName:  loc.PackageName,
		GoModulePath: filepath.Dir(relMod),
		GoModule:     mod.Module.Mod.Path,
		SourcePath:   loc.Rel(pkgName),
		GoVersion:    mod.Go.Version,
	}, nil
}

func GoBuilder(ctx context.Context, pl pkggraph.PackageLoader, loc pkggraph.Location, plan *schema.ImageBuildPlan_GoBuild, unsafeCacheable bool) (build.Spec, error) {
	gobin, err := FromLocation(loc, plan.RelPath)
	if err != nil {
		return nil, fnerrors.AttachLocation(loc, err)
	}

	gobin.BinaryOnly = plan.BinaryOnly
	gobin.BinaryName = plan.BinaryName
	gobin.UnsafeCacheable = unsafeCacheable

	if !plan.BinaryOnly {
		if ok, err := hasGolangBase(ctx, pl); err != nil {
			return nil, err
		} else if !ok {
			return nil, fnerrors.InternalError("the current ns version requires a namespacelabs.dev/foundation dependency with at least version %d", Version_GolangBaseVersionMarker)
		}

		if err := invariants.EnsurePackageLoaded(ctx, pl, loc.PackageName, baseImageRef); err != nil {
			return nil, err
		}
	}

	return gobin, nil
}

func hasGolangBase(ctx context.Context, pl pkggraph.PackageLoader) (bool, error) {
	pkg, err := pl.Resolve(ctx, "namespacelabs.dev/foundation")
	if err != nil {
		return false, err
	}

	data, err := versions.LoadAtOrDefaults(pkg.Module.ReadOnlyFS(), "internal/versions/versions.json")
	if err != nil {
		return false, fnerrors.InternalError("failed to load namespacelabs.dev/foundation version data: %w", err)
	}

	return data.APIVersion >= Version_GolangBaseVersionMarker, nil
}

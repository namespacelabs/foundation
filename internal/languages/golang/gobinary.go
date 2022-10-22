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
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg/knobs"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type GoBinary struct {
	PackageName schema.PackageName `json:"packageName"`

	GoModulePath string `json:"modulePath"` // Relative to workspace root.
	GoModule     string `json:"module"`     // Go module name.
	GoVersion    string `json:"goVersion"`
	SourcePath   string `json:"sourcePath"` // Relative to workspace root.
	BinaryName   string `json:"binaryName"`

	BinaryOnly      bool
	UnsafeCacheable bool // Unsafe because we can't guarantee that the sources used for compilation are consistent with the workspace contents.

	isFocus bool
}

var UseBuildKitForBuilding = knobs.Bool("golang_use_buildkit", "If set to true, buildkit is used for building, instead of a ko-style builder.", false)

func (gb GoBinary) BuildImage(ctx context.Context, env pkggraph.SealedContext, conf build.Configuration) (compute.Computable[oci.Image], error) {
	if conf.PrefersBuildkit() || UseBuildKitForBuilding.Get(env.Configuration()) {
		return buildUsingBuildkit(ctx, env, gb, conf)
	}

	return Build(ctx, env, gb, conf)
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

func GoBuilder(loc pkggraph.Location, plan *schema.ImageBuildPlan_GoBuild, unsafeCacheable bool) (build.Spec, error) {
	gobin, err := FromLocation(loc, plan.RelPath)
	if err != nil {
		return nil, fnerrors.Wrap(loc, err)
	}

	gobin.BinaryOnly = plan.BinaryOnly
	gobin.BinaryName = plan.BinaryName
	gobin.isFocus = plan.IsFocus
	gobin.UnsafeCacheable = unsafeCacheable
	return gobin, nil
}

// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package parsing

import (
	"context"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/parsing/invariants"
	"namespacelabs.dev/foundation/internal/versions"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

const Version_GolangBaseVersionMarker = 68

func transformBinary(ctx context.Context, pl EarlyPackageLoader, loc pkggraph.Location, bin *schema.Binary) error {
	if bin.PackageName != "" {
		return fnerrors.NewWithLocation(loc, "package_name can not be set")
	}

	if bin.Name == "" {
		return fnerrors.NewWithLocation(loc, "binary name can't be empty")
	}

	if bin.BuildPlan == nil {
		return fnerrors.NewWithLocation(loc, "a build plan is required")
	}

	bin.PackageName = loc.PackageName.String()

	if len(bin.GetConfig().GetCommand()) == 0 {
		hasGoLayers := false
		for _, layer := range bin.BuildPlan.LayerBuildPlan {
			if isImagePlanGo(layer) {
				hasGoLayers = true
				break
			}
		}

		// For Go, by default, assume the binary is built with the same name as the package name.
		// TODO: revisit this heuristic.
		if hasGoLayers {
			if bin.Config == nil {
				bin.Config = &schema.BinaryConfig{}
			}

			bin.Config.Command = []string{"/" + bin.Name}
		}
	}

	for _, layer := range bin.BuildPlan.GetLayerBuildPlan() {
		if err := ensureBaseImageDeps(ctx, pl, loc.PackageName, layer); err != nil {
			return err
		}
	}

	return nil
}

func isImagePlanGo(plan *schema.ImageBuildPlan) bool {
	return plan.GoBuild != nil || plan.GoPackage != ""
}

func ensureBaseImageDeps(ctx context.Context, pl EarlyPackageLoader, owner schema.PackageName, layer *schema.ImageBuildPlan) error {
	switch {
	case layer.Binary != nil:
		return invariants.EnsurePackageLoaded(ctx, pl, owner, layer.Binary)

	case layer.GoBuild != nil, layer.GoPackage != "":
		if ok, err := hasGolangBase(ctx, pl); err != nil {
			return err
		} else if !ok {
			return fnerrors.InternalError("the current ns version requires a namespacelabs.dev/foundation dependency with at least version %d", Version_GolangBaseVersionMarker)
		}

		// XXX dedup
		baseImageRef := schema.MakePackageRef("namespacelabs.dev/foundation/library/golang/baseimage", "baseimage")
		return invariants.EnsurePackageLoaded(ctx, pl, owner, baseImageRef)

	case layer.FilesFrom.GetFrom() != nil:
		return ensureBaseImageDeps(ctx, pl, owner, layer.FilesFrom.From)

	case layer.MakeFsImage.GetFrom() != nil:
		return ensureBaseImageDeps(ctx, pl, owner, layer.MakeFsImage.From)

	case layer.MakeSquashfs.GetFrom() != nil:
		return ensureBaseImageDeps(ctx, pl, owner, layer.MakeSquashfs.From)
	}

	return nil
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

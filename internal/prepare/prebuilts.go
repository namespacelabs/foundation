// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package prepare

import (
	"context"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/build/assets"
	"namespacelabs.dev/foundation/internal/build/binary"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/internal/runtime/tools"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/tasks"
)

func DownloadPrebuilts(env pkggraph.SealedContext, packages []schema.PackageName) compute.Computable[[]oci.ResolvableImage] {
	return compute.Map(
		tasks.Action("prepare.download-prebuilts").HumanReadablef("Download prebuilt package images"),
		compute.Inputs().Proto("env", env.Environment()).Strs("packages", schema.Strs(packages...)),
		compute.Output{NotCacheable: true},
		func(ctx context.Context, _ compute.Resolved) ([]oci.ResolvableImage, error) {
			platform, err := tools.HostPlatform(ctx, env.Configuration())
			if err != nil {
				return nil, err
			}

			pl := parsing.NewPackageLoader(env)

			var pkgs []*pkggraph.Package
			var bins []*schema.Binary
			for _, pkg := range packages {
				p, bin, err := pkggraph.LoadBinary(ctx, pl, schema.MakePackageSingleRef(pkg))
				if err != nil {
					return nil, err
				}

				pkgs = append(pkgs, p)
				bins = append(bins, bin)
			}

			sealed := pl.Seal()

			var images []compute.Computable[oci.ResolvableImage]
			for k, p := range pkgs {
				prepared, err := binary.PlanBinary(ctx, pl, env, p.Location, bins[k], assets.AvailableBuildAssets{}, binary.BuildImageOpts{
					UsePrebuilts: true,
					Platforms:    []specs.Platform{platform},
				})
				if err != nil {
					return nil, err
				}

				image, err := prepared.Image(ctx, pkggraph.MakeSealedContext(env, sealed))
				if err != nil {
					return nil, err
				}

				images = append(images, image)
			}

			collectAll := compute.Collect(tasks.Action("prepare.download-prebuilt-images"), images...)
			resolved, err := compute.GetValue(ctx, collectAll)
			if err != nil {
				return nil, err
			}

			var results []oci.ResolvableImage
			for _, r := range resolved {
				results = append(results, r.Value)
			}
			return results, nil
		})
}

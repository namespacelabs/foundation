// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package prepare

import (
	"context"

	"namespacelabs.dev/foundation/build/binary"
	"namespacelabs.dev/foundation/engine/compute"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func DownloadPrebuilts(env pkggraph.SealedContext, packages []schema.PackageName) compute.Computable[[]oci.ResolvableImage] {
	pl := workspace.NewPackageLoader(env)

	return compute.Map(
		tasks.Action("prepare.download-prebuilts").HumanReadablef("Download prebuilt package images"),
		compute.Inputs().Proto("env", env.Environment()).Strs("packages", schema.Strs(packages...)),
		compute.Output{NotCacheable: true},
		func(ctx context.Context, _ compute.Resolved) ([]oci.ResolvableImage, error) {
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
				prepared, err := binary.PlanBinary(ctx, pl, env, p.Location, bins[k], binary.BuildImageOpts{UsePrebuilts: true})
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

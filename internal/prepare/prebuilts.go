// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package prepare

import (
	"context"

	"namespacelabs.dev/foundation/build/binary"
	"namespacelabs.dev/foundation/engine/compute"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/runtime/tools"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func DownloadPrebuilts(env pkggraph.SealedContext, pl pkggraph.PackageLoader, packages []schema.PackageName) compute.Computable[[]oci.Image] {
	return compute.Map(
		tasks.Action("prepare.download-prebuilts").HumanReadablef("Download prebuilt package images"),
		compute.Inputs().Proto("env", env.Environment()).Strs("packages", schema.Strs(packages...)),
		compute.Output{NotCacheable: true},
		func(ctx context.Context, _ compute.Resolved) ([]oci.Image, error) {
			hostPlatform, err := tools.HostPlatform(ctx, env.Configuration())
			if err != nil {
				return nil, err
			}

			var images []compute.Computable[oci.Image]
			for _, pkg := range packages {
				p, err := pl.LoadByName(ctx, pkg)
				if err != nil {
					return nil, err
				}

				// TODO: support defining multiple prebuilt binaries per package
				prebuiltName := ""

				prepared, err := binary.PlanImage(ctx, p, prebuiltName, env, true, &hostPlatform)
				if err != nil {
					return nil, err
				}
				images = append(images, prepared.Image)
			}
			collectAll := compute.Collect(tasks.Action("prepare.download-prebuilt-images"), images...)
			resolved, err := compute.GetValue(ctx, collectAll)
			if err != nil {
				return nil, err
			}
			var results []oci.Image
			for _, r := range resolved {
				results = append(results, r.Value)
			}
			return results, nil
		})
}

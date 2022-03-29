// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package multiplatform

import (
	"context"
	"fmt"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func PrepareMultiPlatformImage(ctx context.Context, env ops.Environment, p build.Plan) (compute.Computable[oci.ResolvableImage], error) {
	img, err := prepareImage(ctx, env, p)
	if err != nil {
		return nil, err
	}

	return compute.Sticky(tasks.Action("build").HumanReadablef(prefix("Build", p.SourceLabel)).Scope(p.SourcePackage), img), nil
}

func prefix(p, label string) string {
	if label == "" {
		return ""
	}
	return p + " " + label
}

func prepareImage(ctx context.Context, env ops.Environment, p build.Plan) (compute.Computable[oci.ResolvableImage], error) {
	if p.Spec.PlatformIndependent() {
		img, err := p.Spec.BuildImage(ctx, env, build.Configuration{
			SourceLabel: p.SourceLabel,
			Workspace:   p.Workspace,
		})
		if err != nil {
			return nil, err
		}
		return oci.WrapImage(img), nil
	}

	platforms := build.PlatformsOrOverrides(p.Platforms)
	if len(platforms) == 0 {
		return nil, fnerrors.InternalError("no platform specified?")
	}

	r, err := prepareMultiPlatformPlan(ctx, p, platforms)
	if err != nil {
		return nil, err
	}

	var images []compute.Computable[oci.Image]
	for _, br := range r.requests {
		image, err := p.Spec.BuildImage(ctx, env, br.Configuration)
		if err != nil {
			return nil, err
		}
		images = append(images, image)
	}

	if len(r.platformIndex) == 1 {
		return oci.WrapImage(images[0]), nil
	}

	var iwp []oci.ImageWithPlatform

	for k, brIndex := range r.platformIndex {
		iwp = append(iwp, oci.ImageWithPlatform{
			Image:    images[brIndex],
			Platform: platforms[k],
		})
	}

	img := oci.MakeImageIndex(iwp...)

	return img, nil
}

type buildRequest struct {
	build.Configuration
	Spec build.Spec
}

type indexPlan struct {
	requests      []buildRequest
	platformIndex []int // Index to build request.
}

func prepareMultiPlatformPlan(ctx context.Context, plan build.Plan, platforms []specs.Platform) (*indexPlan, error) {
	var requests []buildRequest
	var platformIndex []int

	if plan.Spec.PlatformIndependent() {
		br := buildRequest{
			Spec: plan.Spec,
			Configuration: build.Configuration{
				SourceLabel: plan.SourceLabel,
				Workspace:   plan.Workspace,
				Target:      nil, // Plan says it is agnostic.
			},
		}
		requests = append(requests, br)

		for range platforms {
			platformIndex = append(platformIndex, 0) // All platforms point to single build.
		}
	} else {
		for _, plat := range platforms {
			label := plan.SourceLabel
			if len(platforms) > 1 {
				label += fmt.Sprintf(" (%s)", devhost.FormatPlatform(plat))
			}

			br := buildRequest{
				Spec: plan.Spec,
				Configuration: build.Configuration{
					SourceLabel: label,
					Workspace:   plan.Workspace,
					Target:      platformPtr(plat),
				},
			}

			platformIndex = append(platformIndex, len(requests))
			requests = append(requests, br)
		}
	}

	return &indexPlan{requests: requests, platformIndex: platformIndex}, nil
}

func platformPtr(platform specs.Platform) *specs.Platform {
	return &platform
}
// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package multiplatform

import (
	"context"
	"fmt"
	"strings"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/types/known/timestamppb"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/build"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/parsing/devhost"
	"namespacelabs.dev/foundation/internal/storedrun"
	"namespacelabs.dev/foundation/schema/storage"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/tasks"
)

func PrepareMultiPlatformImage(ctx context.Context, env pkggraph.SealedContext, p build.Plan) (compute.Computable[oci.ResolvableImage], error) {
	img, err := prepareImage(ctx, env, p)
	if err != nil {
		return nil, err
	}

	if p.BuildKind != storage.Build_KIND_UNKNOWN && p.SourcePackage != "" {
		img = compute.Consume(tasks.Action("build.attach-results"), img, func(ctx context.Context, v compute.ResultWithTimestamp[oci.ResolvableImage]) error {
			d, _ := v.Value.Digest()

			tasks.Attachments(ctx).
				AddResult("package_name", p.SourcePackage).
				AddResult("digest", d.String()).
				AddResult("cached", v.Cached)

			var platforms []string
			for _, plat := range p.Platforms {
				platforms = append(platforms, devhost.FormatPlatform(plat))
			}

			storedrun.Attach(&storage.Build{
				ActionId:  v.ActionID.String(),
				Package:   p.SourcePackage.String(),
				Kind:      p.BuildKind,
				Cached:    v.Cached,
				Started:   timestamppb.New(v.Started),
				Completed: timestamppb.New(v.Completed),
				Platform:  platforms,
			})

			return nil
		})
	}

	return compute.Sticky(tasks.Action("build").HumanReadablef(prefix("Build", p.SourceLabel)).Scope(p.SourcePackage), img), nil
}

func prefix(p, label string) string {
	if label == "" {
		return ""
	}
	return p + " " + label
}

func prepareImage(ctx context.Context, env pkggraph.SealedContext, plan build.Plan) (compute.Computable[oci.ResolvableImage], error) {
	if plan.Spec.PlatformIndependent() {
		img, err := plan.Spec.BuildImage(ctx, env, build.NewBuildTarget(nil).
			WithTargetName(plan.PublishName).
			WithWorkspace(plan.Workspace).
			WithSourcePackage(plan.SourcePackage).
			WithSourceLabel(plan.SourceLabel))
		if err != nil {
			return nil, err
		}
		return oci.AsResolvable(img), nil
	}

	platforms := build.PlatformsOrOverrides(plan.Platforms)
	if len(platforms) == 0 {
		return nil, fnerrors.InternalError("no platform specified?")
	}

	// Sort platforms, so we yield a stable image order.
	platforms = slices.Clone(platforms)
	slices.SortFunc(platforms, func(a, b specs.Platform) bool {
		return strings.Compare(devhost.FormatPlatform(a), devhost.FormatPlatform(b)) < 0
	})

	r, err := prepareMultiPlatformPlan(ctx, plan, platforms)
	if err != nil {
		return nil, err
	}

	var images []oci.NamedImage
	for _, br := range r.requests {
		image, err := plan.Spec.BuildImage(ctx, env, br.Configuration)
		if err != nil {
			return nil, err
		}
		images = append(images, oci.MakeNamedImage(plan.SourceLabel, image))
	}

	if len(r.platformIndex) == 1 {
		return oci.AsResolvable(images[0].Image()), nil
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
	Configuration build.Configuration
	Spec          build.Spec
}

type indexPlan struct {
	requests      []buildRequest
	platformIndex []int // Index to build request.
}

func prepareMultiPlatformPlan(ctx context.Context, plan build.Plan, platforms []specs.Platform) (*indexPlan, error) {
	var requests []buildRequest
	var platformIndex []int

	for _, plat := range platforms {
		label := plan.SourceLabel
		if len(platforms) > 1 {
			label += fmt.Sprintf(" (%s)", devhost.FormatPlatform(plat))
		}

		platformIndex = append(platformIndex, len(requests))
		requests = append(requests, buildRequest{
			Spec: plan.Spec,
			Configuration: build.NewBuildTarget(platformPtr(plat)).
				WithTargetName(plan.PublishName).
				WithWorkspace(plan.Workspace).
				WithSourcePackage(plan.SourcePackage).
				WithSourceLabel(label),
		})
	}

	return &indexPlan{requests: requests, platformIndex: platformIndex}, nil
}

func platformPtr(platform specs.Platform) *specs.Platform {
	return &platform
}

// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package buildkit

import (
	"context"
	"fmt"

	"github.com/moby/buildkit/client/llb"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/build"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/parsing/platform"
	"namespacelabs.dev/foundation/std/tasks"
)

type ExportToRegistryRequest struct {
	Name     string
	Insecure bool
}

type BuildkitAwareRegistry interface {
	CheckExportRequest(*GatewayClient, oci.AllocatedRepository) (*ExportToRegistryRequest, *ExportToRegistryRequest)
}

func BuildDefinitionToImage(makeClient ClientFactory, conf build.BuildTarget, def *llb.Definition) compute.Computable[oci.Image] {
	return MakeImage(makeClient, conf, precomputedReq(&FrontendRequest{Def: def}, conf), nil, nil)
}

func BuildImage(ctx context.Context, makeClient ClientFactory, conf build.BuildTarget, state llb.State, localDirs ...LocalContents) (compute.Computable[oci.Image], error) {
	serialized, err := MarshalForTarget(ctx, state, conf)
	if err != nil {
		return nil, err
	}

	return MakeImage(makeClient, conf, precomputedReq(&FrontendRequest{Def: serialized, OriginalState: &state}, conf), localDirs, conf.PublishName()), nil
}

type reqToImage struct {
	*baseRequest[oci.Image]

	// If set, targetName will resolve into the allocated image name that this
	// image will be uploaded to, by our caller.
	targetName compute.Computable[oci.AllocatedRepository]
}

func (l *reqToImage) Action() *tasks.ActionEvent {
	ev := tasks.Action("buildkit.build-image")

	if l.targetPlatform != nil {
		ev = ev.Arg("platform", platform.FormatPlatform(*l.targetPlatform))
	}

	if l.sourceLabel != "" {
		ev = ev.HumanReadablef(fmt.Sprintf("Build: %s", l.sourceLabel))
	}

	if l.sourcePackage != "" {
		return ev.Scope(l.sourcePackage)
	}

	return ev
}

func (l *reqToImage) Compute(ctx context.Context, deps compute.Resolved) (oci.Image, error) {
	// TargetName is not added as a dependency of the `reqToImage` compute node, or
	// our inputs are not stable.
	c, err := l.makeClient.MakeClient(ctx)
	if err != nil {
		return nil, err
	}

	if l.targetName != nil {
		v, err := compute.GetValue(ctx, l.targetName)
		if err != nil {
			return nil, err
		}

		requested := ExportToRegistryRequest{v.Repository, v.InsecureRegistry}

		if v.Parent != nil {
			// Enable a docker to docker optimization: if the registry is
			// co-located in the docker network, we may need to rewrite the name
			// we're pushing to, in order to ensure that the image ends up in
			// the registry.
			if bar, ok := v.Parent.(BuildkitAwareRegistry); ok {
				req, transformed := bar.CheckExportRequest(c, v)
				if req != nil {
					requested = *req
				}
				if transformed != nil {
					fmt.Fprintf(console.Debug(ctx), "buildkit: exporting to transformed registry: %q -> %v\n", v.Repository, transformed)
					return l.solve(ctx, c, deps, v.Keychain, exportToRegistry(v.Parent, requested, transformed, v.RegistryAccess))
				}
			}
		}

		if !v.InsecureRegistry {
			if ForwardKeychain {
				return l.solve(ctx, c, deps, v.Keychain, exportToRegistry(v.Parent, requested, nil, v.RegistryAccess))
			} else if v.Keychain == nil {
				// If the target needs permissions, we don't do the direct push
				// optimization as we don't yet wire the keychain into buildkit.
				tasks.Attachments(ctx).AddResult("push", v.Repository)

				return l.solve(ctx, c, deps, nil, exportToRegistry(v.Parent, requested, nil, v.RegistryAccess))
			}
		}
	}

	return l.solve(ctx, c, deps, nil, exportToImage(c.BuildkitOpts()))
}

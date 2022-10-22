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
	"namespacelabs.dev/foundation/internal/parsing/devhost"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/tasks"
)

func BuildDefinitionToImage(env cfg.Context, conf build.BuildTarget, def *llb.Definition) compute.Computable[oci.Image] {
	return makeImage(env, conf, precomputedReq(&frontendReq{Def: def}), nil, nil)
}

func BuildImage(ctx context.Context, env cfg.Context, conf build.BuildTarget, state llb.State, localDirs ...LocalContents) (compute.Computable[oci.Image], error) {
	serialized, err := state.Marshal(ctx)
	if err != nil {
		return nil, err
	}

	return makeImage(env, conf, precomputedReq(&frontendReq{Def: serialized, OriginalState: &state}), localDirs, conf.PublishName()), nil
}

type reqToImage struct {
	*baseRequest[oci.Image]

	// If set, targetName will resolve into the allocated image name that this
	// image will be uploaded to, by our caller.
	targetName compute.Computable[oci.AllocatedName]
}

func (l *reqToImage) Action() *tasks.ActionEvent {
	ev := tasks.Action("buildkit.build-image").
		Arg("platform", devhost.FormatPlatform(l.targetPlatform))

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

	if l.targetName != nil {
		v, err := compute.GetValue(ctx, l.targetName)
		if err != nil {
			return nil, err
		}

		if !v.InsecureRegistry {
			if ForwardKeychain {
				return l.solve(ctx, deps, v.Keychain, exportToRegistry(v.Repository, v.InsecureRegistry))
			} else if v.Keychain == nil {
				// If the target needs permissions, we don't do the direct push
				// optimization as we don't yet wire the keychain into buildkit.
				tasks.Attachments(ctx).AddResult("push", v.Repository)

				img, err := l.solve(ctx, deps, nil, exportToRegistry(v.Repository, v.InsecureRegistry))
				if err != nil {
					return nil, console.WithLogs(ctx, err)
				}
				return img, err
			}
		}
	}

	return l.solve(ctx, deps, nil, exportToImage())
}

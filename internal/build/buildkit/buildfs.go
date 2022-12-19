// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package buildkit

import (
	"context"
	"io/fs"

	"github.com/moby/buildkit/client/llb"
	"namespacelabs.dev/foundation/internal/build"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/parsing/platform"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/tasks"
)

func MarshalForTarget(ctx context.Context, state llb.State, target build.BuildTarget) (*llb.Definition, error) {
	if target.TargetPlatform() == nil {
		return nil, fnerrors.InternalError("target platform is missing")
	}

	return state.Marshal(ctx, llb.Platform(*target.TargetPlatform()))
}

func BuildFilesystem(ctx context.Context, makeClient ClientFactory, target build.BuildTarget, state llb.State, localDirs ...LocalContents) (compute.Computable[fs.FS], error) {
	serialized, err := MarshalForTarget(ctx, state, target)
	if err != nil {
		return nil, err
	}

	base := &baseRequest[fs.FS]{
		sourceLabel:    target.SourceLabel(),
		sourcePackage:  target.SourcePackage(),
		makeClient:     makeClient,
		targetPlatform: target.TargetPlatform(),
		req:            precomputedReq(&FrontendRequest{Def: serialized, OriginalState: &state}, target),
		localDirs:      localDirs,
	}
	return &reqToFS{baseRequest: base}, nil
}

type Input struct {
	State   llb.State
	Secrets []*schema.PackageRef
}

func DeferBuildFilesystem(makeClient ClientFactory, secrets runtime.GroundedSecrets, target build.BuildTarget, state compute.Computable[*Input], localDirs ...LocalContents) compute.Computable[fs.FS] {
	base := &baseRequest[fs.FS]{
		sourceLabel:    target.SourceLabel(),
		sourcePackage:  target.SourcePackage(),
		makeClient:     makeClient,
		targetPlatform: target.TargetPlatform(),
		localDirs:      localDirs,
		secrets:        secrets,
		req: compute.Transform("marshal-request", state, func(ctx context.Context, state *Input) (*FrontendRequest, error) {
			serialized, err := MarshalForTarget(ctx, state.State, target)
			if err != nil {
				return nil, err
			}

			return &FrontendRequest{Def: serialized, OriginalState: &state.State, Secrets: state.Secrets}, nil
		}),
	}
	return &reqToFS{baseRequest: base}
}

type reqToFS struct {
	*baseRequest[fs.FS]
}

func (l *reqToFS) Action() *tasks.ActionEvent {
	ev := tasks.Action("buildkit.build-fs")

	if l.targetPlatform != nil {
		ev = ev.Arg("platform", platform.FormatPlatform(*l.targetPlatform))
	}

	if l.sourcePackage != "" {
		return ev.Scope(l.sourcePackage)
	}

	return ev
}

func (l *reqToFS) Compute(ctx context.Context, deps compute.Resolved) (fs.FS, error) {
	c, err := l.makeClient.MakeClient(ctx)
	if err != nil {
		return nil, err
	}

	return l.solve(ctx, c, deps, nil, exportToFS())
}

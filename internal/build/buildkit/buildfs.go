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
	"namespacelabs.dev/foundation/internal/parsing/platform"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/tasks"
)

func BuildFilesystem(ctx context.Context, conf cfg.Configuration, target build.BuildTarget, state llb.State, localDirs ...LocalContents) (compute.Computable[fs.FS], error) {
	serialized, err := state.Marshal(ctx)
	if err != nil {
		return nil, err
	}

	base := &baseRequest[fs.FS]{
		sourceLabel:    target.SourceLabel(),
		sourcePackage:  target.SourcePackage(),
		config:         conf,
		targetPlatform: platformOrDefault(target.TargetPlatform()),
		req:            precomputedReq(&FrontendRequest{Def: serialized, OriginalState: &state}),
		localDirs:      localDirs,
	}
	return &reqToFS{baseRequest: base}, nil
}

type reqToFS struct {
	*baseRequest[fs.FS]
}

func (l *reqToFS) Action() *tasks.ActionEvent {
	ev := tasks.Action("buildkit.build-fs").
		Arg("platform", platform.FormatPlatform(l.targetPlatform))

	if l.sourcePackage != "" {
		return ev.Scope(l.sourcePackage)
	}

	return ev
}

func (l *reqToFS) Compute(ctx context.Context, deps compute.Resolved) (fs.FS, error) {
	c, err := compute.GetValue(ctx, MakeClient(l.config, l.targetPlatform))
	if err != nil {
		return nil, err
	}

	return l.solve(ctx, c, deps, nil, exportToFS())
}

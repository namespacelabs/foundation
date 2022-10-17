// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package buildkit

import (
	"context"
	"io/fs"

	"github.com/moby/buildkit/client/llb"
	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/parsing/devhost"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/std/tasks"
)

func BuildFilesystem(ctx context.Context, conf planning.Configuration, target build.BuildTarget, state llb.State, localDirs ...LocalContents) (compute.Computable[fs.FS], error) {
	serialized, err := state.Marshal(ctx)
	if err != nil {
		return nil, err
	}

	base := &baseRequest[fs.FS]{
		sourceLabel:    target.SourceLabel(),
		sourcePackage:  target.SourcePackage(),
		config:         conf,
		targetPlatform: platformOrDefault(target.TargetPlatform()),
		req:            precomputedReq(&frontendReq{Def: serialized}),
		localDirs:      localDirs,
	}
	return &reqToFS{baseRequest: base}, nil
}

type reqToFS struct {
	*baseRequest[fs.FS]
}

func (l *reqToFS) Action() *tasks.ActionEvent {
	ev := tasks.Action("buildkit.build-fs").
		Arg("platform", devhost.FormatPlatform(l.targetPlatform))

	if l.sourcePackage != "" {
		return ev.Scope(l.sourcePackage)
	}

	return ev
}

func (l *reqToFS) Compute(ctx context.Context, deps compute.Resolved) (fs.FS, error) {
	return l.solve(ctx, deps, nil, exportToFS())
}

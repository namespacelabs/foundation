// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package binary

import (
	"context"
	"fmt"

	"github.com/moby/buildkit/client/llb"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/build/buildkit"
	"namespacelabs.dev/foundation/internal/llbutil"
	"namespacelabs.dev/foundation/internal/production"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/workspace/pins"
)

const (
	appRootPath   = "/app"
	RunScriptPath = appRootPath + "/ns_run_node.sh"
)

type nodeJsBinary struct {
	nodejsEnv string
}

func (n nodeJsBinary) LLB(ctx context.Context, bnj buildNodeJS, conf build.Configuration) (llb.State, []buildkit.LocalContents, error) {
	nodeImage, err := pins.CheckDefault("node")
	if err != nil {
		return llb.State{}, nil, err
	}

	buildBase, err := prepareNodejsBaseWithPkgManager(ctx, nodeImage, *conf.TargetPlatform())
	if err != nil {
		return llb.State{}, nil, err
	}

	local := buildkit.LocalContents{Module: bnj.loc.Module, Path: bnj.loc.Rel(), ObserveChanges: bnj.isFocus}

	src := buildkit.MakeLocalState(local)

	buildBase = prepareAndRunInstall(ctx, buildBase, src)

	buildBase, err = runBuild(ctx, bnj.loc, buildBase, src)
	if err != nil {
		return llb.State{}, nil, err
	}

	buildBase = addRunScript(ctx, buildBase)

	var out llb.State
	// The dev and prod builds are different:
	//  - For prod we produce the smallest image, without the package manager and its dependencies.
	//  - For dev we keep the base image with the package manager and install nodemon there.
	// This can cause discrepancies between environments however the risk seems to be small.
	if bnj.isDevBuild {
		out = buildBase
	} else {
		// For non-dev builds creating an optimized, small image.
		// buildBase and prodBase must have compatible libcs, e.g. both must be glibc or musl.
		out = llbutil.Image(nodeImage, *conf.TargetPlatform()).With(
			production.NonRootUser(),
			llbutil.CopyFrom(buildBase, appRootPath, appRootPath),
		)
	}

	out = out.AddEnv("NODE_ENV", n.nodejsEnv)

	return out, []buildkit.LocalContents{local}, nil
}

func prepareNodejsBaseWithPkgManager(ctx context.Context, nodejsBase string, platform specs.Platform) (llb.State, error) {
	base := llbutil.Image(nodejsBase, platform)

	// TODO: detect and install the package manager (npm, yarn, pnpm, etc.).

	return base, nil
}

func prepareAndRunInstall(ctx context.Context, base llb.State, src llb.State) llb.State {
	state := base.
		File(
			llb.Mkdir(appRootPath, 0644)).
		With(
			llb.Dir(appRootPath),
			llbutil.CopyFrom(src, "./package.json", "."))

	// TODO: use the detected package manager.
	state = state.Run(llb.Shlex("npm install")).Root()

	return state
}

func runBuild(ctx context.Context, loc pkggraph.Location, base llb.State, src llb.State) (llb.State, error) {
	state := base.With(llbutil.CopyFrom(src, ".", "."))

	pkgJson, err := readPackageJson(loc)
	if err != nil {
		return llb.State{}, err
	}

	if _, ok := pkgJson.Scripts["build"]; ok {
		// TODO: use the detected package manager.
		state = state.Run(llb.Shlex("npm run build")).Root()
	}

	return state, nil
}

func addRunScript(ctx context.Context, base llb.State) llb.State {
	return llbutil.AddFile(base, RunScriptPath, 0755, []byte(genRunScript()))
}

// We generate a run script so the container command can be static.
func genRunScript() string {
	// TODO: use the detected package manager.
	return fmt.Sprintf(`#!/bin/sh
cd %s
npm start`, appRootPath)
}

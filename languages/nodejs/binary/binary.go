// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package binary

import (
	"context"

	"github.com/moby/buildkit/client/llb"
	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/build/buildkit"
	"namespacelabs.dev/foundation/internal/llbutil"
	"namespacelabs.dev/foundation/internal/production"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/workspace/pins"
)

const (
	AppRootPath = "/app"
)

var (
	NodejsExclude = []string{"**/.yarn", "**/.pnp.*"}
)

type nodeJsBinary struct {
	nodejsEnv string
}

func (n nodeJsBinary) LLB(ctx context.Context, bnj buildNodeJS, conf build.Configuration) (llb.State, []buildkit.LocalContents, error) {
	nodeImage, err := pins.CheckDefault("node")
	if err != nil {
		return llb.State{}, nil, err
	}

	local := buildkit.LocalContents{Module: bnj.loc.Module, Path: bnj.loc.Rel(), ObserveChanges: bnj.isFocus}
	src := buildkit.MakeCustomLocalState(local, buildkit.MakeLocalStateOpts{
		Exclude: NodejsExclude,
	})

	pkgMgrRuntime, err := pkgMgrToRuntime(local, *conf.TargetPlatform(), bnj.nodePkgMgr)
	if err != nil {
		return llb.State{}, nil, err
	}

	buildBase := llbutil.Image(nodeImage, *conf.TargetPlatform())
	buildBase = prepareAndRunInstall(ctx, pkgMgrRuntime, buildBase, src)

	buildBase, err = runBuild(ctx, pkgMgrRuntime.cliName, bnj.loc, buildBase, src)
	if err != nil {
		return llb.State{}, nil, err
	}

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
		out = llbutil.Image(nodeImage, *conf.TargetPlatform()).
			With(pkgMgrRuntime.installCliWithConfigFiles,
				production.NonRootUser(),
				llbutil.CopyFrom(buildBase, AppRootPath, AppRootPath),
			)
	}

	out = out.AddEnv("NODE_ENV", n.nodejsEnv)

	return out, []buildkit.LocalContents{local}, nil
}

func prepareAndRunInstall(ctx context.Context, pkgMgrRuntime pkgMgrRuntime, base llb.State, src llb.State) llb.State {
	return base.
		File(llb.Mkdir(AppRootPath, 0644)).
		With(llb.Dir(AppRootPath), pkgMgrRuntime.installCliWithConfigFiles).
		Run(llb.Shlexf("%s install", pkgMgrRuntime.cliName)).Root()
}

func runBuild(ctx context.Context, pkgMgrCliName string, loc pkggraph.Location, base llb.State, src llb.State) (llb.State, error) {
	state := base.With(llbutil.CopyFrom(src, ".", "."))

	pkgJson, err := readPackageJson(loc)
	if err != nil {
		return llb.State{}, err
	}

	if _, ok := pkgJson.Scripts["build"]; ok {
		state = state.Run(llb.Shlexf("%s run build", pkgMgrCliName)).Root()
	}

	return state, nil
}

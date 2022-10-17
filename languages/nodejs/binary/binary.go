// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package binary

import (
	"context"
	"path/filepath"

	"github.com/moby/buildkit/client/llb"
	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/build/buildkit"
	"namespacelabs.dev/foundation/internal/dependencies/pins"
	"namespacelabs.dev/foundation/internal/llbutil"
	"namespacelabs.dev/foundation/internal/production"
)

const (
	AppRootPath = "/app"
)

var (
	NodejsExclude = []string{"**/.yarn", "**/.pnp.*"}
)

type nodeJsBinary struct {
	nodejsEnv string
	module    build.Workspace
}

func (n nodeJsBinary) LLB(ctx context.Context, bnj buildNodeJS, conf build.Configuration) (llb.State, []buildkit.LocalContents, error) {
	nodeImage, err := pins.CheckDefault("node")
	if err != nil {
		return llb.State{}, nil, err
	}

	local := buildkit.LocalContents{Module: n.module, Path: bnj.loc.Rel(), ObserveChanges: bnj.isFocus}
	src := buildkit.MakeCustomLocalState(local, buildkit.MakeLocalStateOpts{
		Exclude: NodejsExclude,
	})

	packageManagerState, err := handlePackageManager(src, *conf.TargetPlatform(), bnj.config.NodePkgMgr)
	if err != nil {
		return llb.State{}, nil, err
	}

	srcWithPkgMgr := llbutil.Image(nodeImage, *conf.TargetPlatform()).
		File(llb.Mkdir(AppRootPath, 0644)).
		With(llb.Dir(AppRootPath), packageManagerState.makeState).
		Run(llb.Shlexf("%s install", packageManagerState.cli)).Root().
		With(llbutil.CopyFrom(src, ".", "."))

	var out llb.State
	// The dev and prod builds are different:
	//  - For prod we produce the smallest image, without the package manager and its dependencies.
	//  - For dev we keep the base image with the package manager.
	// This can cause discrepancies between environments however the risk seems to be small.
	if bnj.isDevBuild {
		out = srcWithPkgMgr
	} else {
		if bnj.config.BuildScript != "" {
			srcWithPkgMgr = srcWithPkgMgr.Run(llb.Shlexf("%s run %s", packageManagerState.cli, bnj.config.BuildScript)).Root()
		}

		if bnj.config.BuildOutDir != "" {
			// In this case creating an image with just the built files.
			// TODO: do it outside of the Node.js implementation.
			pathToCopy := filepath.Join(AppRootPath, bnj.config.BuildOutDir)

			out = llb.Scratch().With(llbutil.CopyFrom(srcWithPkgMgr, pathToCopy, pathToCopy))
		} else {
			// For non-dev builds creating an optimized, small image.
			// buildBase and prodBase must have compatible libcs, e.g. both must be glibc or musl.
			out = llbutil.Image(nodeImage, *conf.TargetPlatform()).
				With(packageManagerState.makeState,
					production.NonRootUser(),
					llbutil.CopyFrom(srcWithPkgMgr, AppRootPath, AppRootPath),
				)
		}
	}

	out = out.AddEnv("NODE_ENV", n.nodejsEnv)

	return out, []buildkit.LocalContents{local}, nil
}

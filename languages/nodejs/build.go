// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package nodejs

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/moby/buildkit/client/llb"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/build/buildkit"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/llbutil"
	"namespacelabs.dev/foundation/internal/production"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/foundation/workspace/pins"
)

const appRootPath = "/app"
const tmpBundlePath = "/tmp_bundle"

type buildNodeJS struct {
	serverLoc workspace.Location
	deps      []workspace.Location
	isFocus   bool
}

func (bnj buildNodeJS) BuildImage(ctx context.Context, env ops.Environment, conf build.Configuration) (compute.Computable[oci.Image], error) {
	nodeImage, err := pins.CheckDefault("node")
	if err != nil {
		return nil, err
	}

	n := NodeJsBinary{
		NodeJsBase: nodeImage,
		Env:        nodeEnv(env),
	}

	state, local := n.LLB(bnj, conf)

	return buildkit.LLBToImage(ctx, env, conf.Target, state, local)
}

func nodeEnv(env ops.Environment) string {
	if env.Proto().GetPurpose() == schema.Environment_PRODUCTION {
		return "production"
	} else {
		return "development"
	}
}

func (buildNodeJS) PlatformIndependent() bool { return false }

type NodeJsBinary struct {
	NodeJsBase string
	Env        string
}

func (n NodeJsBinary) LLB(bnj buildNodeJS, conf build.Configuration) (llb.State, buildkit.LocalContents) {
	local := buildkit.LocalContents{Module: bnj.serverLoc.Module, Path: "", ObserveChanges: bnj.isFocus}
	src := buildkit.MakeLocalState(local)

	yarnWorkspacePaths := []string{bnj.serverLoc.Rel()}
	for _, dep := range bnj.deps {
		yarnWorkspacePaths = append(yarnWorkspacePaths, dep.Rel())
	}

	buildBase := prepareYarnWithWorkspaces(yarnWorkspacePaths, n.NodeJsBase, src, *conf.Target)

	stateOptions := []llb.StateOption{
		llbutil.CopyFrom(src, "tsconfig.json", filepath.Join(appRootPath, "tsconfig.json")),
		llbutil.CopyFrom(src, "package.json", filepath.Join(appRootPath, "package.json")),
		llbutil.CopyFrom(buildBase, filepath.Join(appRootPath, "node_modules"), filepath.Join(appRootPath, "node_modules")),
	}

	for _, path := range yarnWorkspacePaths {
		stateOptions = append(stateOptions, llbutil.CopyFrom(
			src, path, filepath.Join(appRootPath, path)))
	}

	// buildBase and prodBase must have compatible libcs, e.g. both must be glibc or musl.
	out := production.PrepareImage(llbutil.Image(n.NodeJsBase, *conf.Target), *conf.Target).
		With(stateOptions...).
		AddEnv("NODE_ENV", n.Env).
		Run(llb.Shlex("yarn run build"), llb.Dir(appRootPath)).Root()

	return out, local
}

func prepareYarnWithWorkspaces(workspacePaths []string, nodejsBase string, src llb.State, platform specs.Platform) llb.State {
	base := llbutil.Image(nodejsBase, platform)
	buildBase := base.Run(llb.Shlex("apk add --no-cache python2 make g++")).
		Root().
		AddEnv("YARN_CACHE_FOLDER", "/cache/yarn").
		With(
			llbutil.CopyFrom(src, "package.json", filepath.Join(appRootPath, "package.json")),
			llbutil.CopyFrom(src, "yarn.lock", filepath.Join(appRootPath, "yarn.lock")))
	for _, path := range workspacePaths {
		buildBase = buildBase.With(llbutil.CopyFrom(src, filepath.Join(path, "package.json"), filepath.Join(appRootPath, path, "package.json")))
	}

	yarnInstall := buildBase.Run(llb.Shlex("yarn install --frozen-lockfile"), llb.Dir(appRootPath))
	yarnInstall.AddMount("/cache/yarn", llb.Scratch(), llb.AsPersistentCacheDir(
		"yarn-cache-"+strings.ReplaceAll(devhost.FormatPlatform(platform), "/", "-"), llb.CacheMountShared))

	return yarnInstall.Root()
}

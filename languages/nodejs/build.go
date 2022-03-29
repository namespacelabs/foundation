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

type buildNodeJS struct {
	loc     workspace.Location
	isFocus bool
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

	state, local := n.LLB(bnj.loc, bnj.isFocus, conf)

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

func (n NodeJsBinary) LLB(loc workspace.Location, rebuildOnChanges bool, conf build.Configuration) (llb.State, buildkit.LocalContents) {
	local := buildkit.LocalContents{Module: loc.Module, Path: loc.Rel(), ObserveChanges: rebuildOnChanges}
	src := buildkit.MakeLocalState(local)

	buildBase := PrepareYarn("/app", n.NodeJsBase, src, *conf.Target)

	// buildBase and prodBase must have compatible libcs, e.g. both must be glibc or musl.
	out := production.PrepareImage(llbutil.Image(n.NodeJsBase, *conf.Target), *conf.Target).
		With(
			llbutil.CopyFrom(src, ".", "/app"),
			llbutil.CopyFrom(buildBase, "/app/node_modules", "/app/node_modules")).
		AddEnv("NODE_ENV", n.Env).
		Run(llb.Shlex("yarn run build"), llb.Dir("/app"))

	return out.Root(), local
}

func PrepareYarn(target, nodejsBase string, src llb.State, platform specs.Platform) llb.State {
	base := llbutil.Image(nodejsBase, platform)
	buildBase := base.Run(llb.Shlex("apk add --no-cache python2 make g++")).
		Root().
		AddEnv("YARN_CACHE_FOLDER", "/cache/yarn").
		With(
			llbutil.CopyFrom(src, "package.json", filepath.Join(target, "package.json")),
			llbutil.CopyFrom(src, "yarn.lock", filepath.Join(target, "yarn.lock")))

	yarnInstall := buildBase.Run(llb.Shlex("yarn install --frozen-lockfile"), llb.Dir(target))
	yarnInstall.AddMount("/cache/yarn", llb.Scratch(), llb.AsPersistentCacheDir("yarn-cache-"+strings.ReplaceAll(devhost.FormatPlatform(platform), "/", "-"), llb.CacheMountShared))

	return yarnInstall.Root()
}
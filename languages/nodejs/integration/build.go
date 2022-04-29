// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package integration

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/moby/buildkit/client/llb"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/build/binary"
	"namespacelabs.dev/foundation/build/buildkit"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/llbutil"
	"namespacelabs.dev/foundation/internal/production"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/foundation/workspace/pins"
)

const appRootPath = "/app"

type buildNodeJS struct {
	module     build.Workspace
	locs       []workspace.Location
	yarnRoot   schema.PackageName
	serverEnv  provision.ServerEnv
	isDevBuild bool
	isFocus    bool
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

	nodejsImage, err := buildkit.LLBToImage(ctx, env, conf.Target, state, local)
	if err != nil {
		return nil, err
	}

	if bnj.isDevBuild {
		// Adding dev controller
		pkg, err := bnj.serverEnv.LoadByName(ctx, controllerPkg)
		if err != nil {
			return nil, err
		}

		p, err := binary.Plan(ctx, pkg, binary.BuildImageOpts{UsePrebuilts: true})
		if err != nil {
			return nil, err
		}

		devControllerImage, err := p.Plan.Spec.BuildImage(ctx, env, build.Configuration{
			SourceLabel: p.Plan.SourceLabel,
			Workspace:   p.Plan.Workspace,
			Target:      conf.Target,
		})
		if err != nil {
			return nil, err
		}

		images := []compute.Computable[oci.Image]{nodejsImage, devControllerImage}

		return oci.MergeImageLayers(images...), nil
	} else {

		return nodejsImage, nil
	}
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
	local := buildkit.LocalContents{Module: bnj.module, Path: ".", ObserveChanges: bnj.isFocus}
	src := buildkit.MakeLocalState(local)

	yarnWorkspacePaths := []string{}
	for _, dep := range bnj.locs {
		yarnWorkspacePaths = append(yarnWorkspacePaths, dep.Rel())
	}

	yarnRoot := bnj.yarnRoot.String()
	buildBase := prepareYarnWithWorkspaces(yarnWorkspacePaths, yarnRoot, bnj.isDevBuild, n.NodeJsBase, src, *conf.Target)

	var out llb.State
	// The dev and prod builds are different:
	//  - For prod we produce the smallest image, without Yarn and its dependencies.
	//  - For dev we keep the base image with Yarn and install nodemon there.
	// This can cause discrepancies between environments however the risk seems to be small.
	if bnj.isDevBuild {
		out = buildBase
	} else {
		// For non-dev builds creating an optimized, small image.

		stateOptions := []llb.StateOption{
			llbutil.CopyFrom(buildBase, filepath.Join(appRootPath, yarnRoot, "node_modules"), filepath.Join(appRootPath, yarnRoot, "node_modules")),
		}
		for _, path := range yarnWorkspacePaths {
			stateOptions = append(stateOptions, llbutil.CopyFrom(
				buildBase, filepath.Join(appRootPath, path), filepath.Join(appRootPath, path)))
		}

		// buildBase and prodBase must have compatible libcs, e.g. both must be glibc or musl.
		out = production.PrepareImage(llbutil.Image(n.NodeJsBase, *conf.Target), *conf.Target).
			With(stateOptions...)
	}

	out = out.AddEnv("NODE_ENV", n.Env)

	return out, local
}

func prepareYarnWithWorkspaces(workspacePaths []string, yarnRoot string, isDevBuild bool, nodejsBase string, src llb.State, platform specs.Platform) llb.State {
	base := llbutil.Image(nodejsBase, platform)
	targetYarnRoot := filepath.Join(appRootPath, yarnRoot)
	buildBase := base.Run(llb.Shlex("apk add --no-cache python2 make g++")).
		Root().
		AddEnv("YARN_CACHE_FOLDER", "/cache/yarn")
	if isDevBuild {
		// Nodemon is used to watch for changes in the source code within a container and restart the "ts-node" server.
		buildBase = buildBase.Run(llb.Shlex("yarn global add nodemon@2.0.15 ts-node@10.7.0"), llb.Dir(yarnRoot)).Root()
	}
	for _, fn := range []string{"package.json", "tsconfig.json", "yarn.lock", ".yarnrc.yml", ".yarn/releases"} {
		buildBase = buildBase.With(
			llbutil.CopyFrom(src, filepath.Join(yarnRoot, fn), filepath.Join(targetYarnRoot, fn)))
	}
	for _, path := range workspacePaths {
		buildBase = buildBase.With(llbutil.CopyFrom(src, path, filepath.Join(appRootPath, path)))
	}

	yarnInstall := buildBase.Run(llb.Shlex("yarn install --immutable"), llb.Dir(targetYarnRoot))
	yarnInstall.AddMount("/cache/yarn", llb.Scratch(), llb.AsPersistentCacheDir(
		"yarn-cache-"+strings.ReplaceAll(devhost.FormatPlatform(platform), "/", "-"), llb.CacheMountShared))
	out := yarnInstall.Root()

	// No need to compile Typescript for dev builds, "nodemon" does it itself.
	if !isDevBuild {
		out = out.
			Run(llb.Shlex("yarn plugin import workspace-tools@3.1.1"), llb.Dir(targetYarnRoot)).Root().
			// Compile Typescript in parallel in the reverse dependency order.
			Run(llb.Shlex("yarn workspaces foreach -pt run tsc"), llb.Dir(targetYarnRoot)).Root()
	}

	return out
}

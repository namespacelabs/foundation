// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package web

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
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/internal/fnfs/workspace/wsremote"
	"namespacelabs.dev/foundation/internal/llbutil"
	"namespacelabs.dev/foundation/internal/yarn"
	nodejs "namespacelabs.dev/foundation/languages/nodejs/integration"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/foundation/workspace/pins"
	"namespacelabs.dev/foundation/workspace/tasks"
)

// Returns a Computable[v1.Image] with the results of the compilation.
func ViteBuild(ctx context.Context, loc workspace.Location, env ops.Environment, conf build.BuildTarget, baseOutput, basePath string, extraFiles ...*memfs.FS) (compute.Computable[oci.Image], error) {
	local, base, err := viteBase(ctx, conf, "/app", loc.Module, loc.Rel(), false, extraFiles...)
	if err != nil {
		return nil, err
	}

	if !strings.HasSuffix(basePath, "/") {
		basePath += "/"
	}

	out := base.
		AddEnv("NODE_ENV", "production").
		Run(llb.Shlexf("node_modules/vite/bin/vite.js build --base=%s --outDir=%s --emptyOutDir", basePath, filepath.Join("/out", baseOutput)), llb.Dir("/app")).
		AddMount("/out", llb.Scratch())

	image, err := buildkit.LLBToImage(ctx, env, conf, out, local)
	if err != nil {
		return nil, err
	}

	return compute.Named(tasks.Action("web.vite.build").Arg("builder", "buildkit"), image), nil
}

func viteSource(ctx context.Context, target string, loc workspace.Location, isFocus bool, env ops.Environment, conf build.BuildTarget, extraFiles ...*memfs.FS) (compute.Computable[oci.Image], error) {
	var module build.Workspace

	if r := wsremote.Ctx(ctx); r != nil && isFocus && !loc.Module.IsExternal() {
		module = yarn.YarnHotReloadModule{
			Mod:  loc.Module,
			Sink: r.For(&wsremote.Signature{ModuleName: loc.Module.ModuleName(), Rel: loc.Rel()}),
		}
	} else {
		module = loc.Module
	}

	local, state, err := viteBase(ctx, conf, target, module, loc.Rel(), isFocus, extraFiles...)
	if err != nil {
		return nil, err
	}

	image, err := buildkit.LLBToImage(ctx, env, conf, state, local)
	if err != nil {
		return nil, err
	}

	return compute.Named(tasks.Action("web.vite.build.dev").Arg("builder", "buildkit").Scope(loc.PackageName), image), nil
}

func viteBase(ctx context.Context, conf build.BuildTarget, target string, module build.Workspace, rel string, rebuildOnChanges bool, extraFiles ...*memfs.FS) (buildkit.LocalContents, llb.State, error) {
	local := buildkit.LocalContents{Module: module, Path: rel, ObserveChanges: rebuildOnChanges}

	src := buildkit.MakeLocalState(local)

	nodeImage, err := pins.CheckDefault("node")
	if err != nil {
		return buildkit.LocalContents{}, llb.State{}, err
	}

	buildBase, err := PrepareYarn(ctx, target, nodeImage, src, *conf.TargetPlatform())
	if err != nil {
		return buildkit.LocalContents{}, llb.State{}, err
	}

	// buildBase and prodBase must have compatible libcs, e.g. both must be glibc or musl.
	base := llbutil.Image(nodeImage, *conf.TargetPlatform()).
		With(
			llbutil.CopyFrom(src, ".", target),
			llbutil.CopyFrom(buildBase, filepath.Join(target, "node_modules"), filepath.Join(target, "node_modules")))

	for _, extra := range extraFiles {
		base, err = llbutil.WriteFS(ctx, extra, base, target)
		if err != nil {
			return buildkit.LocalContents{}, llb.State{}, err
		}
	}

	return local, base, nil
}

func PrepareYarn(ctx context.Context, target, nodejsBase string, src llb.State, platform specs.Platform) (llb.State, error) {
	base, err := nodejs.PrepareYarnBase(ctx, nodejsBase, platform)
	if err != nil {
		return llb.State{}, err
	}

	buildBase := base.With(
		llbutil.CopyFrom(src, "package.json", filepath.Join(target, "package.json")),
		llbutil.CopyFrom(src, "yarn.lock", filepath.Join(target, "yarn.lock")))

	yarnInstall := buildBase.Run(nodejs.RunYarnShlex("install", "--immutable"), llb.Dir(target))
	yarnInstall.AddMount("/cache/yarn", llb.Scratch(), llb.AsPersistentCacheDir("yarn-cache-"+strings.ReplaceAll(devhost.FormatPlatform(platform), "/", "-"), llb.CacheMountShared))

	return yarnInstall.Root(), nil
}

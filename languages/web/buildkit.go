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
	"namespacelabs.dev/foundation/internal/nodejs"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/foundation/workspace/pins"
	"namespacelabs.dev/foundation/workspace/tasks"
)

// Returns a Computable[v1.Image] with the results of the compilation.
func ViteProductionBuild(ctx context.Context, loc workspace.Location, env ops.Environment, description, baseOutput, basePath string, extraFiles ...*memfs.FS) (compute.Computable[oci.Image], error) {
	hostPlatform := buildkit.HostPlatform()
	conf := build.NewBuildTarget(&hostPlatform).WithSourceLabel(description)

	local, base, err := viteBuildBase(ctx, conf, "/app", loc.Module, loc.Rel(), false, extraFiles...)
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

func viteDevBuild(ctx context.Context, env ops.Environment, target string, loc workspace.Location, isFocus bool, conf build.BuildTarget, extraFiles ...*memfs.FS) (compute.Computable[oci.Image], error) {
	var module build.Workspace

	if r := wsremote.Ctx(ctx); r != nil && isFocus && !loc.Module.IsExternal() {
		module = nodejs.YarnHotReloadModule{
			Module: loc.Module,
			Sink:   r.For(&wsremote.Signature{ModuleName: loc.Module.ModuleName(), Rel: loc.Rel()}),
		}
	} else {
		module = loc.Module
	}

	local, state, err := viteBuildBase(ctx, conf, target, module, loc.Rel(), isFocus, extraFiles...)
	if err != nil {
		return nil, err
	}

	image, err := buildkit.LLBToImage(ctx, env, conf, state, local)
	if err != nil {
		return nil, err
	}

	return compute.Named(tasks.Action("web.vite.build.dev").Arg("builder", "buildkit").Scope(loc.PackageName), image), nil
}

func viteBuildBase(ctx context.Context, conf build.BuildTarget, target string, module build.Workspace, rel string, rebuildOnChanges bool, extraFiles ...*memfs.FS) (buildkit.LocalContents, llb.State, error) {
	local := buildkit.LocalContents{Module: module, Path: rel, ObserveChanges: rebuildOnChanges}

	src := buildkit.MakeLocalState(local)

	nodeImage, err := pins.CheckDefault("node")
	if err != nil {
		return buildkit.LocalContents{}, llb.State{}, err
	}

	buildBase, err := prepareYarn(ctx, target, nodeImage, src, *conf.TargetPlatform())
	if err != nil {
		return buildkit.LocalContents{}, llb.State{}, err
	}

	// buildBase and prodBase must have compatible libcs, e.g. both must be glibc or musl.
	base := llbutil.Image(nodeImage, *conf.TargetPlatform()).
		With(llbutil.CopyFrom(buildBase, filepath.Join(target, "node_modules"), filepath.Join(target, "node_modules")))

	// Use separate layers for node_modules, and sources, as the latter change more often.
	base = base.With(llbutil.CopyFrom(src, ".", target))

	for _, extra := range extraFiles {
		base, err = llbutil.WriteFS(ctx, extra, base, target)
		if err != nil {
			return buildkit.LocalContents{}, llb.State{}, err
		}
	}

	return local, base, nil
}

func prepareYarn(ctx context.Context, target, nodejsBase string, src llb.State, platform specs.Platform) (llb.State, error) {
	base, err := nodejs.PrepareNodejsBaseWithYarnForBuild(ctx, nodejsBase, platform)
	if err != nil {
		return llb.State{}, err
	}

	buildBase := base.With(
		llbutil.CopyFrom(src, "package.json", filepath.Join(target, "package.json")),
		llbutil.CopyFrom(src, "yarn.lock", filepath.Join(target, "yarn.lock")))

	yarnInstall := buildBase.Run(nodejs.RunYarnShlex("install", "--immutable"), llb.Dir(target))
	yarnInstall.AddMount(nodejs.YarnContainerCacheDir, llb.Scratch(), llb.AsPersistentCacheDir("yarn-cache-"+strings.ReplaceAll(devhost.FormatPlatform(platform), "/", "-"), llb.CacheMountShared))

	return yarnInstall.Root(), nil
}

// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package web

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/moby/buildkit/client/llb"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/build"
	"namespacelabs.dev/foundation/internal/build/buildkit"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/dependencies/pins"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/internal/integrations/nodejs/binary"
	"namespacelabs.dev/foundation/internal/llbutil"
	"namespacelabs.dev/foundation/internal/nodejs"
	"namespacelabs.dev/foundation/internal/parsing/devhost"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/tasks"
)

const yarnLockFn = "yarn.lock"

// Returns a Computable[v1.Image] with the results of the compilation.
func ViteProductionBuild(ctx context.Context, loc pkggraph.Location, env cfg.Context, description, baseOutput, basePath string, externalModules []build.Workspace, extraFiles ...*memfs.FS) (oci.NamedImage, error) {
	hostPlatform := buildkit.HostPlatform()
	conf := build.NewBuildTarget(&hostPlatform).WithSourceLabel(description)

	local, base, err := viteBuildBase(ctx, conf, "/app", loc.Module, loc.Rel(), loc.Module.Workspace, false, externalModules, extraFiles...)
	if err != nil {
		return nil, err
	}

	if !strings.HasSuffix(basePath, "/") {
		basePath += "/"
	}

	out := base.
		AddEnv("NODE_ENV", "production").
		Run(llb.Shlexf("node_modules/vite/bin/vite.js build --base=%s --outDir=%s --emptyOutDir --config=prodweb.config.js", basePath, filepath.Join("/out", baseOutput)), llb.Dir(filepath.Join("/app", loc.Rel()))).
		AddMount("/out", llb.Scratch())

	image, err := buildkit.BuildImage(ctx, env, conf, out, local...)
	if err != nil {
		return nil, err
	}

	return oci.MakeNamedImage(fmt.Sprintf("vite-production-build-%s", loc.PackageName),
		compute.Named(tasks.Action("web.vite.build").Arg("builder", "buildkit"), image)), nil
}

func viteDevBuild(ctx context.Context, env cfg.Context, targetDir string, loc pkggraph.Location, isFocus bool, conf build.Configuration, externalModules []build.Workspace, extraFiles ...*memfs.FS) (oci.NamedImage, error) {
	local, state, err := viteBuildBase(ctx, conf, targetDir, conf.Workspace(), loc.Rel(), loc.Module.Workspace, isFocus, externalModules, extraFiles...)
	if err != nil {
		return nil, err
	}

	image, err := buildkit.BuildImage(ctx, env, conf, state, local...)
	if err != nil {
		return nil, err
	}

	return oci.MakeNamedImage(fmt.Sprintf("vite-dev-build-%s", loc.PackageName),
		compute.Named(tasks.Action("web.vite.build.dev").Arg("builder", "buildkit").Scope(loc.PackageName), image)), nil
}

func viteBuildBase(ctx context.Context, conf build.BuildTarget, target string, module build.Workspace, rel string, workspace *schema.Workspace, rebuildOnChanges bool, externalModules []build.Workspace, extraFiles ...*memfs.FS) ([]buildkit.LocalContents, llb.State, error) {
	nodeImage, err := pins.CheckDefault("node")
	if err != nil {
		return nil, llb.State{}, err
	}

	buildBase, err := nodejs.PrepareNodejsBaseWithYarnForBuild(ctx, nodeImage, *conf.TargetPlatform())
	if err != nil {
		return nil, llb.State{}, err
	}

	local := buildkit.LocalContents{
		Module:          module,
		Path:            ".",
		ObserveChanges:  rebuildOnChanges,
		TemporaryIsWeb:  true,
		ExcludePatterns: binary.NodejsExclude,
	}

	locals, buildBase, err := nodejs.AddExternalModules(ctx, workspace, rel, buildBase, externalModules)
	if err != nil {
		return nil, llb.State{}, err
	}
	locals = append(locals, local)

	src := buildkit.MakeLocalState(local)

	buildBase, err = runYarn(ctx, rel, target, buildBase, src, *conf.TargetPlatform())
	if err != nil {
		return nil, llb.State{}, err
	}

	// buildBase and prodBase must have compatible libcs, e.g. both must be glibc or musl.
	base := llbutil.Image(nodeImage, *conf.TargetPlatform()).
		With(llbutil.CopyFrom(buildBase, filepath.Join(target, rel, "node_modules"), filepath.Join(target, rel, "node_modules")))
	if len(externalModules) > 0 {
		base = base.With(llbutil.CopyFrom(buildBase, nodejs.DepsRootPath, nodejs.DepsRootPath))
	}

	// Use separate layers for node_modules, and sources, as the latter change more often.
	base = base.With(llbutil.CopyFrom(src, ".", target))

	for _, extra := range extraFiles {
		base, err = llbutil.WriteFS(ctx, extra, base, filepath.Join(target, rel))
		if err != nil {
			return nil, llb.State{}, err
		}
	}

	return locals, base, nil
}

func runYarn(ctx context.Context, rel string, targetDir string, base llb.State, src llb.State, platform specs.Platform) (llb.State, error) {
	buildBase := base.With(llbutil.CopyFrom(src, ".", filepath.Join(targetDir)))

	yarnInstall := buildBase.Run(nodejs.RunYarnShlex("install", "--immutable"), llb.Dir(filepath.Join(targetDir, rel)))
	yarnInstall.AddMount(nodejs.YarnContainerCacheDir, llb.Scratch(), llb.AsPersistentCacheDir("yarn-cache-"+strings.ReplaceAll(devhost.FormatPlatform(platform), "/", "-"), llb.CacheMountShared))

	return yarnInstall.Root(), nil
}

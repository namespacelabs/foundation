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
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/internal/fnfs/workspace/wsremote"
	"namespacelabs.dev/foundation/internal/llbutil"
	"namespacelabs.dev/foundation/internal/wscontents"
	"namespacelabs.dev/foundation/languages/nodejs"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/pins"
	"namespacelabs.dev/foundation/workspace/tasks"
)

// Returns a Computable[v1.Image] with the results of the compilation.
func ViteBuild(ctx context.Context, loc workspace.Location, env ops.Environment, targetPlatform *specs.Platform, baseOutput, basePath string, extraFiles ...*memfs.FS) (compute.Computable[oci.Image], error) {
	local, base, err := viteBase(ctx, "/app", loc.Module, loc.Rel(), false, extraFiles...)
	if err != nil {
		return nil, err
	}

	if !strings.HasSuffix(basePath, "/") {
		basePath += "/"
	}

	out := base.
		AddEnv("NODE_ENV", "production").
		Run(llb.Shlexf("node_modules/.bin/vite build --base=%s --outDir=%s --emptyOutDir", basePath, filepath.Join("/out", baseOutput)), llb.Dir("/app")).
		AddMount("/out", llb.Scratch())

	image, err := buildkit.LLBToImage(ctx, env, targetPlatform, out, local)
	if err != nil {
		return nil, err
	}

	return compute.Named(tasks.Action("web.vite.build").Arg("builder", "buildkit"), image), nil
}

func viteSource(ctx context.Context, target string, loc workspace.Location, isFocus bool, env ops.Environment, targetPlatform *specs.Platform, extraFiles ...*memfs.FS) (compute.Computable[oci.Image], error) {
	var module build.Workspace

	if r := wsremote.Ctx(ctx); r != nil && isFocus && !loc.Module.IsExternal() {
		module = webModule{
			mod:  loc.Module,
			sink: r.For(&wsremote.Signature{ModuleName: loc.Module.ModuleName(), Rel: loc.Rel()}),
		}
	} else {
		module = loc.Module
	}

	local, state, err := viteBase(ctx, target, module, loc.Rel(), isFocus, extraFiles...)
	if err != nil {
		return nil, err
	}

	image, err := buildkit.LLBToImage(ctx, env, targetPlatform, state, local)
	if err != nil {
		return nil, err
	}

	return compute.Named(tasks.Action("web.vite.build.dev").Arg("builder", "buildkit").Scope(loc.PackageName), image), nil
}

func viteBase(ctx context.Context, target string, module build.Workspace, rel string, rebuildOnChanges bool, extraFiles ...*memfs.FS) (buildkit.LocalContents, llb.State, error) {
	local := buildkit.LocalContents{Module: module, Path: rel, ObserveChanges: rebuildOnChanges}

	src := buildkit.MakeLocalState(local)

	nodeImage, err := pins.CheckDefault("node")
	if err != nil {
		return buildkit.LocalContents{}, llb.State{}, err
	}

	buildBase := nodejs.PrepareYarn(target, nodeImage, src, buildkit.HostPlatform())

	// buildBase and prodBase must have compatible libcs, e.g. both must be glibc or musl.
	base := llbutil.Image(nodeImage, buildkit.HostPlatform()).
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

type webModule struct {
	mod  *workspace.Module
	sink wsremote.Sink
}

func (w webModule) ModuleName() string { return w.mod.ModuleName() }
func (w webModule) Abs() string        { return w.mod.Abs() }
func (w webModule) VersionedFS(rel string, observeChanges bool) compute.Computable[wscontents.Versioned] {
	if observeChanges {
		return wsremote.ObserveAndPush(w.mod.Abs(), rel, observerSink{w.sink})
	}

	return w.mod.VersionedFS(rel, observeChanges)
}

type observerSink struct {
	sink wsremote.Sink
}

func (obs observerSink) Deposit(ctx context.Context, events []*wscontents.FileEvent) error {
	for _, ev := range events {
		if ev.Path == "yarn.lock" {
			return fnerrors.ExpectedError("yarn.lock changed, triggering a rebuild")
		}
	}

	return obs.sink.Deposit(ctx, events)
}
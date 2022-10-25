// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package binary

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/moby/buildkit/client/llb"
	"namespacelabs.dev/foundation/internal/build"
	"namespacelabs.dev/foundation/internal/build/buildkit"
	"namespacelabs.dev/foundation/internal/bytestream"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/dependencies/pins"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/internal/llbutil"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
)

const (
	AppRootPath      = "/app"
	backendsConfigFn = "src/config/backends.ns.js"
)

var (
	NodejsExclude = []string{"**/.yarn/cache", "**/.pnp.*"}
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

	packageManagerState, err := LookupPackageManager(bnj.config.NodePkgMgr)
	if err != nil {
		return llb.State{}, nil, err
	}

	devImage := llbutil.Image(nodeImage, *conf.TargetPlatform()).
		AddEnv("NODE_ENV", "development").
		File(llb.Mkdir(AppRootPath, 0644))

	if packageManagerState.MakeState != nil {
		devImage = devImage.With(packageManagerState.MakeState)
	}

	fsys, err := compute.GetValue(ctx, conf.Workspace().VersionedFS(bnj.loc.Rel(), false))
	if err != nil {
		return llb.State{}, nil, err
	}

	local := buildkit.LocalContents{
		Module:          n.module,
		Path:            bnj.loc.Rel(),
		ObserveChanges:  bnj.isFocus,
		ExcludePatterns: NodejsExclude,
	}
	src := buildkit.MakeLocalState(local)

	devImage, err = copySrcForInstall(ctx, devImage, src, fsys.FS(), packageManagerState)
	if err != nil {
		return llb.State{}, nil, err
	}

	devImage = devImage.
		Run(
			llb.Shlexf("%s install", packageManagerState.CLI),
			llb.Dir(AppRootPath),
		).
		With(llbutil.CopyFrom(src, ".", AppRootPath))

	devImage, err = maybeGenerateBackendsJs(ctx, devImage, bnj)
	if err != nil {
		return llb.State{}, nil, err
	}

	var out llb.State
	if bnj.isDevBuild {
		out = devImage
	} else {
		prodCfg := bnj.config.Prod

		var prodImage llb.State
		if prodCfg.InstallDeps {
			prodImage = llbutil.Image(nodeImage, *conf.TargetPlatform()).
				AddEnv("NODE_ENV", "production").
				File(llb.Mkdir(AppRootPath, 0644))

			prodImage, err = copySrcForInstall(ctx, prodImage, src, fsys.FS(), packageManagerState)
			if err != nil {
				return llb.State{}, nil, err
			}

			prodImage = prodImage.Run(
				llb.Shlexf("%s install", packageManagerState.CLI),
				llb.Dir(AppRootPath),
			).Root()

		} else {
			prodImage = llb.Scratch()
		}

		if prodCfg.BuildScript != "" {
			devImage = devImage.
				// Important to build in the prod mode.
				AddEnv("NODE_ENV", "production").
				Run(
					llb.Shlexf("%s run %s", packageManagerState.CLI, prodCfg.BuildScript),
					llb.Dir(AppRootPath),
				).Root()
		}

		pathToCopy := filepath.Join(AppRootPath, prodCfg.BuildOutDir)
		destPath := pathToCopy
		if prodCfg.BuildOutDir == "" {
			destPath = "."
		}

		prodImage = prodImage.With(llbutil.CopyFromExcluding(devImage, pathToCopy, destPath,
			append(packageManagerState.ExcludePatterns, dirs.BasePatternsToExclude...)))

		out = prodImage
	}

	return out, []buildkit.LocalContents{local}, nil
}

func maybeGenerateBackendsJs(ctx context.Context, base llb.State, bnj buildNodeJS) (llb.State, error) {
	if len(bnj.config.InternalDoNotUseBackend) == 0 {
		return base, nil
	}

	// XXX replace with general purpose `genrule` layer.
	if _, err := fs.Stat(bnj.loc.Module.ReadOnlyFS(), bnj.loc.Rel(backendsConfigFn)); os.IsNotExist(err) {
		bytes, err := generateBackendsConfig(ctx, bnj.loc, bnj.config.InternalDoNotUseBackend, bnj.assets.IngressFragments, true /* placeholder */)
		if err != nil {
			return llb.State{}, err
		}

		return base, fnerrors.UserError(bnj.loc, `%q must be present in the source tree when Web backends are used. Example content:

%s
`, backendsConfigFn, bytes)
	}

	bytes, err := generateBackendsConfig(ctx, bnj.loc, bnj.config.InternalDoNotUseBackend, bnj.assets.IngressFragments, false /* placeholder */)
	if err != nil {
		return llb.State{}, err
	}

	var fsys memfs.FS
	fsys.Add(backendsConfigFn, bytes)

	return llbutil.WriteFS(ctx, &fsys, base, AppRootPath)
}

// Copies package.json and other files from "src" that are needed for the "install" call.
func copySrcForInstall(ctx context.Context, base llb.State, src llb.State, fsys fs.FS, packageManagerState *PackageManager) (llb.State, error) {
	opts := fnfs.MatcherOpts{
		IncludeFiles:      append([]string{"package.json"}, packageManagerState.RequiredFiles...),
		ExcludeFilesGlobs: packageManagerState.ExcludePatterns,
	}

	for _, wc := range packageManagerState.WildcardDirectories {
		opts.IncludeFilesGlobs = append(opts.IncludeFilesGlobs, wc+"/**/*")
	}

	m, err := fnfs.NewMatcher(opts)
	if err != nil {
		return llb.State{}, err
	}

	if err := fnfs.VisitFiles(ctx, fsys, func(path string, bs bytestream.ByteStream, _ fs.DirEntry) error {
		if !m.Excludes(path) && m.Includes(path) {
			base = base.With(llbutil.CopyFrom(src, path, filepath.Join(AppRootPath, path)))
		}
		return nil
	}); err != nil {
		return llb.State{}, err
	}

	return base, nil
}

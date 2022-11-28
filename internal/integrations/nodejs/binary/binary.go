// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package binary

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/moby/buildkit/client/llb"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/internal/build"
	"namespacelabs.dev/foundation/internal/build/buildkit"
	"namespacelabs.dev/foundation/internal/bytestream"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/dependencies/pins"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/internal/integrations/opaque"
	"namespacelabs.dev/foundation/internal/llbutil"
	"namespacelabs.dev/foundation/internal/parsing/devhost"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
	"namespacelabs.dev/foundation/schema"
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
}

func (n nodeJsBinary) LLB(ctx context.Context, bnj buildNodeJS, conf build.Configuration) (llb.State, []buildkit.LocalContents, error) {
	packageManagerState, err := LookupPackageManager(bnj.config.NodePkgMgr)
	if err != nil {
		return llb.State{}, nil, err
	}

	fsys, err := compute.GetValue(ctx, conf.Workspace().VersionedFS(bnj.loc.Rel(), false))
	if err != nil {
		return llb.State{}, nil, err
	}

	local := buildkit.LocalContents{
		Module:          conf.Workspace(),
		Path:            bnj.loc.Rel(),
		ObserveChanges:  bnj.isFocus,
		ExcludePatterns: NodejsExclude,
	}
	src := buildkit.MakeLocalState(local)

	var platform specs.Platform
	if conf.TargetPlatform() != nil {
		platform = *conf.TargetPlatform()
	} else {
		// Happens for Web builds where the output is platform-independent.
		platform = buildkit.HostPlatform()
	}

	devImage, err := createBaseImageAndInstallYarn(ctx, platform, src, fsys.FS(), "development", packageManagerState)
	if err != nil {
		return llb.State{}, nil, err
	}

	devImage = devImage.With(llbutil.CopyFrom(src, ".", AppRootPath))

	devImage, err = maybeGenerateBackendsJs(ctx, devImage, bnj)
	if err != nil {
		return llb.State{}, nil, err
	}

	var out llb.State
	if opaque.UseDevBuild(bnj.env) {
		out = devImage
	} else {
		prodCfg := bnj.config.Prod

		var prodImage llb.State
		if prodCfg.InstallDeps {
			prodImage, err = createBaseImageAndInstallYarn(ctx, platform, src, fsys.FS(), "production", packageManagerState)
			if err != nil {
				return llb.State{}, nil, err
			}
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
		bytes, err := generateBackendsConfig(ctx, bnj.loc, bnj.config.InternalDoNotUseBackend, bnj.assets.IngressFragments,
			&BackendsOpts{Placeholder: true})
		if err != nil {
			return llb.State{}, err
		}

		return base, fnerrors.NewWithLocation(bnj.loc, `%q must be present in the source tree when Web backends are used. Example content:

%s
`, backendsConfigFn, bytes)
	}

	bytes, err := generateBackendsConfig(ctx, bnj.loc, bnj.config.InternalDoNotUseBackend, bnj.assets.IngressFragments,
		&BackendsOpts{Placeholder: false, UseInClusterAddresses: bnj.env.Purpose == schema.Environment_TESTING})
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

func createBaseImageAndInstallYarn(ctx context.Context, platform specs.Platform, src llb.State, fsys fs.FS, nodeEnv string, packageManagerState *PackageManager) (llb.State, error) {
	nodeImage, err := pins.CheckDefault("node")
	if err != nil {
		return llb.State{}, err
	}

	baseImage := llbutil.Image(nodeImage, platform).
		AddEnv("NODE_ENV", nodeEnv).
		File(llb.Mkdir(AppRootPath, 0644))

	if packageManagerState.MakeState != nil {
		baseImage = baseImage.With(packageManagerState.MakeState)
	}

	baseImage, err = copySrcForInstall(ctx, baseImage, src, fsys, packageManagerState)
	if err != nil {
		return llb.State{}, err
	}

	plfrm := strings.ReplaceAll(devhost.FormatPlatform(platform), "/", "-")

	pkgMgrInstall := baseImage.
		Run(
			llb.Shlexf(packageManagerState.InstallCmd),
			llb.Dir(AppRootPath),
		)
	pkgMgrInstall.AddMount(filepath.Join(containerCacheDir, packageManagerState.CacheKey), llb.Scratch(),
		llb.AsPersistentCacheDir(fmt.Sprintf("%s-cache-%s-%s", packageManagerState.CacheKey, nodeEnv, plfrm),
			// Not using a shared cache: it causes transient failures when the same files
			// are accessed by multiple package manager instances.
			llb.CacheMountPrivate))

	return pkgMgrInstall.Root(), nil
}

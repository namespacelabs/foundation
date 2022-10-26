// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package binary

import (
	"path/filepath"

	"github.com/moby/buildkit/client/llb"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
)

var (
	npmFiles  = []string{".npmrc", "package-lock.json"}
	yarnFiles = []string{"yarn.lock", ".yarnrc.yml"}
	pnpmFiles = []string{"pnpm-lock.yaml", ".npmrc", ".pnpmfile.cjs"}

	packageManagerSources = makeAllFiles(npmFiles, yarnFiles, pnpmFiles)

	containerCacheDir = "/cache"
)

type PackageManager struct {
	CLI                 string
	InstallCmd          string
	CacheKey            string
	MakeState           llb.StateOption
	RequiredFiles       []string // Files patterns which are relevant to this package manager.
	WildcardDirectories []string
	ExcludePatterns     []string
}

func LookupPackageManager(pkgMgr schema.NodejsBuild_NodePkgMgr) (*PackageManager, error) {
	switch pkgMgr {
	case schema.NodejsBuild_NPM:
		cacheKey := "npm"
		return &PackageManager{
			CLI:        "npm",
			InstallCmd: "npm install",
			CacheKey:   cacheKey,
			MakeState: func(base llb.State) llb.State {
				// Not installing the "npm" binary itself: relying on the base version built into the "node:alpine" image.
				return base.
					Run(llb.Shlexf("npm config set cache %q --global", filepath.Join(containerCacheDir, cacheKey))).
					Run(llb.Shlexf("npm config set update-notifier false")).Root()

			},
			RequiredFiles: npmFiles,
		}, nil

	case schema.NodejsBuild_YARN, schema.NodejsBuild_YARN3:
		// Not installing "yarn v1" itself: relying on the base version built into the "node:alpine" image.

		pm := &PackageManager{
			CLI:           "yarn",
			RequiredFiles: yarnFiles,
		}

		if pkgMgr == schema.NodejsBuild_YARN3 {
			// Note: yarn v3 always installs "dev" dependencies: https://github.com/yarnpkg/berry/issues/2253

			pm.WildcardDirectories = []string{".yarn"}
			pm.ExcludePatterns = []string{".yarn/cache/**"}
			pm.InstallCmd = "yarn install --immutable"
			pm.CacheKey = "yarn3"
			pm.MakeState = func(base llb.State) llb.State {
				return base.
					AddEnv("YARN_GLOBAL_FOLDER", filepath.Join(containerCacheDir, pm.CacheKey)).
					AddEnv("YARN_ENABLE_GLOBAL_CACHE", "true")
			}
		} else {
			pm.InstallCmd = "yarn install --frozen-lockfile --non-interactive"
			pm.CacheKey = "yarn1"
			pm.MakeState = func(base llb.State) llb.State {
				return base.AddEnv("YARN_CACHE_FOLDER", filepath.Join(containerCacheDir, pm.CacheKey))
			}
		}

		return pm, nil

	case schema.NodejsBuild_PNPM:
		cacheKey := "pnpm"
		return &PackageManager{
			CLI:        "pnpm",
			InstallCmd: "pnpm install",
			CacheKey:   cacheKey,
			MakeState: func(base llb.State) llb.State {
				return base.
					Run(llb.Shlexf("npm --no-update-notifier --no-fund --global install pnpm@%s", versions().Pnpm)).
					Run(llb.Shlexf("pnpm config set update-notifier false")).
					// Shared cache to speed up builds.
					Run(llb.Shlexf("pnpm config set store-dir %q", filepath.Join(containerCacheDir, cacheKey))).
					// The caches is on a different disk so hardlinks won't work.
					Run(llb.Shlexf("pnpm config set package-import-method copy")).Root()
			},
			RequiredFiles: pnpmFiles,
		}, nil
	}

	return nil, fnerrors.InternalError("unknown nodejs package manager: %v", pkgMgr)
}

func makeAllFiles(s ...[]string) []string {
	x := []string{"package.json"}
	for _, s := range s {
		x = append(x, s...)
	}
	return x
}

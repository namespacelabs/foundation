// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package binary

import (
	"github.com/moby/buildkit/client/llb"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
)

var (
	npmFiles  = []string{".npmrc", "package-lock.json"}
	yarnFiles = []string{"yarn.lock", ".yarnrc.yml"}
	pnpmFiles = []string{"pnpm-lock.yaml", ".npmrc", ".pnpmfile.cjs"}

	packageManagerSources = makeAllFiles(npmFiles, yarnFiles, pnpmFiles)
)

type PackageManager struct {
	CLI                 string
	MakeState           llb.StateOption
	RequiredFiles       []string // Files patterns which are relevant to this package manager.
	WildcardDirectories []string
	ExcludePatterns     []string
}

func LookupPackageManager(pkgMgr schema.NodejsIntegration_NodePkgMgr) (*PackageManager, error) {
	switch pkgMgr {
	case schema.NodejsIntegration_NPM:
		return &PackageManager{
			CLI: "npm",
			// Not installing the "npm" binary itself: relying on the base version built into the "node:alpine" image.
			MakeState:     nil,
			RequiredFiles: npmFiles,
		}, nil

	case schema.NodejsIntegration_YARN, schema.NodejsIntegration_YARN3:
		pm := &PackageManager{
			CLI: "yarn",
			// Not installing "yarn v1" itself: relying on the base version built into the "node:alpine" image.
			MakeState:     nil,
			RequiredFiles: yarnFiles,
		}

		if pkgMgr == schema.NodejsIntegration_YARN3 {
			pm.WildcardDirectories = []string{".yarn"}
			pm.ExcludePatterns = []string{".yarn/cache/**"}
		}

		return pm, nil

	case schema.NodejsIntegration_PNPM:
		return &PackageManager{
			CLI: "pnpm",
			MakeState: func(base llb.State) llb.State {
				return base.Run(llb.Shlexf("npm --no-update-notifier --no-fund --global install pnpm@%s", versions().Pnpm)).Root()
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

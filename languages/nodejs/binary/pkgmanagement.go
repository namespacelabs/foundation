// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package binary

import (
	"github.com/moby/buildkit/client/llb"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/llbutil"
	"namespacelabs.dev/foundation/schema"
)

var (
	npmFiles  = []string{".npmrc", "package-lock.json"}
	yarnFiles = []string{"yarn.lock", ".yarnrc.yml", ".yarn/releases", ".yarn/plugins", ".yarn/patches", ".yarn/versions"}
	pnpmFiles = []string{"pnpm-lock.yaml", ".npmrc", ".pnpmfile.cjs"}

	packageManagerSources = makeAllFiles(npmFiles, yarnFiles, pnpmFiles)
)

type packageManager struct {
	cli       string
	makeState llb.StateOption
}

func PackageManagerCLI(pkgMgr schema.NodejsIntegration_NodePkgMgr) (string, error) {
	switch pkgMgr {
	case schema.NodejsIntegration_NPM:
		return "npm", nil
	case schema.NodejsIntegration_YARN:
		return "yarn", nil
	case schema.NodejsIntegration_PNPM:
		return "pnpm", nil
	default:
		return "", fnerrors.InternalError("unknown nodejs package manager: %v", pkgMgr)
	}
}

func handlePackageManager(workspace llb.State, platform specs.Platform, pkgMgr schema.NodejsIntegration_NodePkgMgr) (*packageManager, error) {
	switch pkgMgr {
	case schema.NodejsIntegration_NPM:
		return &packageManager{
			cli: "npm",
			// Not installing the "npm" binary itself: relying on the base version built into the "node:alpine" image.
			makeState: llbutil.CopyPatterns(workspace, append([]string{"package.json"}, npmFiles...), "."),
		}, nil

	case schema.NodejsIntegration_YARN:
		return &packageManager{
			cli: "yarn",
			// Not installing "yarn v1" itself: relying on the base version built into the "node:alpine" image.
			makeState: llbutil.CopyPatterns(workspace, append([]string{"package.json"}, yarnFiles...), "."),
		}, nil

	case schema.NodejsIntegration_PNPM:
		return &packageManager{
			cli: "pnpm",
			makeState: func(base llb.State) llb.State {
				withPnpm := base.Run(llb.Shlexf("npm --no-update-notifier --no-fund --global install pnpm@%s", versions().Pnpm)).Root()

				return withPnpm.With(
					llbutil.CopyPatterns(workspace, append([]string{"package.json"}, pnpmFiles...), "."),
				)
			},
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

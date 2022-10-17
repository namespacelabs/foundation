// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package binary

import (
	"github.com/moby/buildkit/client/llb"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
)

var (
	npmFiles  = []string{".npmrc", "package-lock.json"}
	yarnFiles = []string{"yarn.lock", ".yarnrc.yml"}
	pnpmFiles = []string{"pnpm-lock.yaml", ".npmrc", ".pnpmfile.cjs"}

	packageManagerSources = makeAllFiles(npmFiles, yarnFiles, pnpmFiles)
)

type packageManager struct {
	CLI                 string
	State               llb.StateOption
	FilePatterns        []string // Files patterns which are relevant to this package manager.
	WildcardDirectories []string
	ExcludePatterns     []string
}

func PackageManagerCLI(pkgMgr schema.NodejsIntegration_NodePkgMgr) (string, error) {
	switch pkgMgr {
	case schema.NodejsIntegration_NPM:
		return "npm", nil
	case schema.NodejsIntegration_YARN, schema.NodejsIntegration_YARN3:
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
			CLI: "npm",
			// Not installing the "npm" binary itself: relying on the base version built into the "node:alpine" image.
			State:        nil,
			FilePatterns: npmFiles,
		}, nil

	case schema.NodejsIntegration_YARN, schema.NodejsIntegration_YARN3:
		pm := &packageManager{
			CLI: "yarn",
			// Not installing "yarn v1" itself: relying on the base version built into the "node:alpine" image.
			State:        nil,
			FilePatterns: yarnFiles,
		}

		if pkgMgr == schema.NodejsIntegration_YARN3 {
			pm.WildcardDirectories = []string{".yarn"}
			pm.ExcludePatterns = []string{".yarn/cache/**"}
		}

		return pm, nil

	case schema.NodejsIntegration_PNPM:
		return &packageManager{
			CLI: "pnpm",
			State: func(base llb.State) llb.State {
				return base.Run(llb.Shlexf("npm --no-update-notifier --no-fund --global install pnpm@%s", versions().Pnpm)).Root()
			},
			FilePatterns: pnpmFiles,
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

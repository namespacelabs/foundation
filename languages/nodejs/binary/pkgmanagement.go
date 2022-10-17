// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package binary

import (
	"github.com/moby/buildkit/client/llb"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/internal/dependencies/pins"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/llbutil"
	"namespacelabs.dev/foundation/schema"
)

var (
	npmFiles  = []string{".npmrc", "package-lock.json"}
	yarnFiles = []string{"yarn.lock", ".yarnrc.yml", ".yarn/releases", ".yarn/plugins", ".yarn/patches", ".yarn/versions"}
	pnpmFiles = []string{"pnpm-lock.yaml"}

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
		alpineName, err := pins.CheckDefault("alpine")
		if err != nil {
			return nil, err
		}

		pnpmPath := "/bin/pnpm"
		pnpmBase := llbutil.Image(alpineName, platform).
			Run(llb.Shlex("apk add --no-cache curl")).Root().
			Run(llb.Shlexf(`curl -fsSL "https://github.com/pnpm/pnpm/releases/download/v%s/pnpm-linuxstatic-x64" -o %s`,
				versions().Pnpm, pnpmPath)).Root().
			Run(llb.Shlexf("chmod +x %s", pnpmPath)).Root()

		return &packageManager{
			cli: "pnpm",
			makeState: func(base llb.State) llb.State {
				return base.With(
					llbutil.CopyFrom(pnpmBase, pnpmPath, pnpmPath),
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

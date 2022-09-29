// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package binary

import (
	"github.com/moby/buildkit/client/llb"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/build/buildkit"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/llbutil"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/pins"
)

var (
	// Paths of files required for installing dependencies. Also changes to them trigger a full rebuild.
	pathsForBuild = []string{
		// Common
		"package.json",
		// Npm
		".npmrc", "package-lock.json",
		// Yarn
		".yarnrc.yml", ".yarn/releases", ".yarn/plugins", ".yarn/patches", ".yarn/versions", "yarn.lock",
		// Pnpm
		"pnpm-lock.yaml",
	}
	patternsForBuild = pathsToPatterns(pathsForBuild)
)

type pkgMgrRuntime struct {
	cliName                   string
	installCliWithConfigFiles func(llb.State) llb.State
}

func PkgMgrCliName(pkgMgr schema.NodejsIntegration_NodePkgMgr) (string, error) {
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

func pkgMgrToRuntime(local buildkit.LocalContents, platform specs.Platform, pkgMgr schema.NodejsIntegration_NodePkgMgr) (pkgMgrRuntime, error) {
	configsSrc := buildkit.MakeCustomLocalState(local, buildkit.MakeLocalStateOpts{
		Include: patternsForBuild,
	})
	cliName, err := PkgMgrCliName(pkgMgr)
	if err != nil {
		return pkgMgrRuntime{}, err
	}

	runtime := pkgMgrRuntime{
		cliName: cliName,
	}

	switch pkgMgr {
	case schema.NodejsIntegration_NPM:
		runtime.installCliWithConfigFiles = func(base llb.State) llb.State {
			// Not installing the "npm" binary itself: relying on the base version built into the "node:alpine" image.
			return base.With(llbutil.CopyFrom(configsSrc, ".", "."))
		}
	case schema.NodejsIntegration_YARN:
		runtime.installCliWithConfigFiles = func(base llb.State) llb.State {
			// Not installing "yarn v1" itself: relying on the base version built into the "node:alpine" image.
			return base.With(llbutil.CopyFrom(configsSrc, ".", "."))
		}
	case schema.NodejsIntegration_PNPM:
		alpineName, err := pins.CheckDefault("alpine")
		if err != nil {
			return pkgMgrRuntime{}, err
		}

		pnpmPath := "/bin/pnpm"
		pnpmBase := llbutil.Image(alpineName, platform).
			Run(llb.Shlex("apk add --no-cache curl")).Root().
			Run(llb.Shlexf(`curl -fsSL "https://github.com/pnpm/pnpm/releases/download/v%s/pnpm-linuxstatic-x64" -o %s`,
				versions().Pnpm, pnpmPath)).Root().
			Run(llb.Shlexf("chmod +x %s", pnpmPath)).Root()

		runtime.installCliWithConfigFiles = func(base llb.State) llb.State {
			return base.With(llbutil.CopyFrom(pnpmBase, pnpmPath, pnpmPath)).
				With(llbutil.CopyFrom(configsSrc, ".", "."))
		}
	default:
		return pkgMgrRuntime{}, fnerrors.InternalError("unknown nodejs package manager: %v", pkgMgr)
	}

	return runtime, nil
}

func pathsToPatterns(paths []string) []string {
	patterns := make([]string, len(paths))
	for i, path := range paths {
		patterns[i] = "**/" + path
	}
	return patterns
}

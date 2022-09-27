// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package binary

import (
	"io/fs"
	"path/filepath"

	"github.com/moby/buildkit/client/llb"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/build/buildkit"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/llbutil"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/workspace/pins"
)

const (
	npmLockfile  = "package-lock.json"
	yarnLockfile = "yarn.lock"
	pnpmLockfile = "pnpm-lock.yaml"
	pnpmPath     = "/bin/pnpm"
)

var (
	// Paths of files required for installing dependencies. Also changes to them trigger a full rebuild.
	pathsForBuild = []string{
		// Common
		"package.json",
		// Npm
		".npmrc", npmLockfile,
		// Yarn
		".yarnrc.yml", ".yarn/releases", ".yarn/plugins", ".yarn/patches", ".yarn/versions", yarnLockfile,
		// Pnpm
		pnpmLockfile,
	}
	patternsForBuild = pathsToPatterns(pathsForBuild)
)

type pkgMgr interface {
	// Install the package manager CLI and any config files (e.g. ".yarnrc.yml") required for installing dependencies.
	InstallCliWithConfigFiles(llb.State) llb.State
	CliName() string
}

func detectPkgMgr(platform specs.Platform, local buildkit.LocalContents, loc pkggraph.Location, fsys fs.FS) (pkgMgr, error) {
	configsSrc := buildkit.MakeCustomLocalState(local, buildkit.MakeLocalStateOpts{
		Include: patternsForBuild,
	})

	if _, err := fs.Stat(fsys, filepath.Join(loc.Rel(), npmLockfile)); err == nil {
		return npmPkgMgr{configsSrc}, nil
	}
	if _, err := fs.Stat(fsys, filepath.Join(loc.Rel(), yarnLockfile)); err == nil {
		return yarnPkgMgr{configsSrc}, nil
	}
	if _, err := fs.Stat(fsys, filepath.Join(loc.Rel(), pnpmLockfile)); err == nil {
		return newPnpmPkgMgr(platform, configsSrc)
	}

	return nil, fnerrors.UserError(loc, "no package manager detected")
}

type npmPkgMgr struct {
	configsSrc llb.State
}

func (npm npmPkgMgr) InstallCliWithConfigFiles(base llb.State) llb.State {
	// Not installing "npm" itself: relying on the base version built into the "node:alpine" image.
	return base.With(llbutil.CopyFrom(npm.configsSrc, ".", "."))
}
func (npmPkgMgr) CliName() string { return "npm" }

type yarnPkgMgr struct {
	src llb.State
}

func (yarn yarnPkgMgr) InstallCliWithConfigFiles(base llb.State) llb.State {
	// Not installing "yarn v1" itself: relying on the base version built into the "node:alpine" image.
	return base.With(llbutil.CopyFrom(yarn.src, ".", "."))
}
func (yarnPkgMgr) CliName() string { return "yarn" }

type pnpmPkgMgr struct {
	src  llb.State
	base llb.State
}

func newPnpmPkgMgr(platform specs.Platform, src llb.State) (pnpmPkgMgr, error) {
	alpineName, err := pins.CheckDefault("alpine")
	if err != nil {
		return pnpmPkgMgr{}, err
	}

	base := llbutil.Image(alpineName, platform).
		Run(llb.Shlex("apk add --no-cache curl")).Root().
		Run(llb.Shlexf(`curl -fsSL "https://github.com/pnpm/pnpm/releases/download/v%s/pnpm-linuxstatic-x64" -o %s`,
			versions().Pnpm, pnpmPath)).Root().
		Run(llb.Shlexf("chmod +x %s", pnpmPath)).Root()

	return pnpmPkgMgr{src, base}, nil
}

// Relying on the version built into the "node:alpine" image.
func (p pnpmPkgMgr) InstallCliWithConfigFiles(base llb.State) llb.State {
	return base.With(llbutil.CopyFrom(p.base, pnpmPath, pnpmPath)).
		With(llbutil.CopyFrom(p.src, ".", "."))
}
func (pnpmPkgMgr) CliName() string { return pnpmPath }

func pathsToPatterns(paths []string) []string {
	patterns := make([]string, len(paths))
	for i, path := range paths {
		patterns[i] = "**/" + path
	}
	return patterns
}

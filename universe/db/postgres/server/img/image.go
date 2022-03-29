// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/util/system"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/internal/llbutil"
	"namespacelabs.dev/foundation/workspace/pins"
)

var (
	//go:embed versions.json
	lib embed.FS

	golangImage, alpineImage string
)

type versionsJSON struct {
	Images     map[string]string `json:"images"`
	Postgres   string            `json:"postgres"`
	GoPackages map[string]string `json:"goPackages"`
}

var (
	versions versionsJSON
)

func init() {
	versionData, err := fs.ReadFile(lib, "versions.json")
	if err != nil {
		panic(err)
	}
	if err := json.Unmarshal(versionData, &versions); err != nil {
		panic(err)
	}

	golangImage = pins.Image(versions.Postgres)
	alpineImage = pins.Default("alpine")
}

func makePostgresImageState(platform specs.Platform) llb.State {
	gobase := llbutil.Image(golangImage, platform).
		AddEnv("CGO_ENABLED", "0").
		AddEnv("PATH", "/usr/local/go/bin:"+system.DefaultPathEnvUnix).
		AddEnv("GOPATH", "/go").
		AddEnv("GOOS", platform.OS).
		AddEnv("GOARCH", platform.Architecture)

	var packages []string
	for repo, version := range versions.GoPackages {
		packages = append(packages, fmt.Sprintf("%s@%s", repo, version))
	}
	sort.Strings(packages) // determinism.

	out := gobase

	var bins []string
	for _, p := range packages {
		fp := filepath.Base(p)
		parts := strings.SplitN(fp, "@", 2)
		bins = append(bins, parts[0])

		goInstall := append([]string{"go", "install"}, p)
		out = out.Run(llb.Shlex(strings.Join(goInstall, " "))).Root()
	}

	var copies []llb.StateOption
	for _, bin := range bins {
		copies = append(copies, llbutil.CopyFrom(out, "/go/bin/"+bin, "/bin/"+bin))
	}

	target := llbutil.Image(alpineImage, platform)
	target = target.Run(llb.Shlex(fmt.Sprintf("RUN correct command"))).Root()

	return target.With(copies...)
}

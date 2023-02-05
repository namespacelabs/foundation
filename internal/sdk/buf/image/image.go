// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package image

import (
	"context"
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
	"namespacelabs.dev/foundation/internal/dependencies/pins"
	"namespacelabs.dev/foundation/internal/llbutil"
	"namespacelabs.dev/foundation/internal/production"
)

var (
	//go:embed versions.json
	lib embed.FS
)

type versionsJSON struct {
	Images     map[string]string `json:"images"`
	Buf        bufDef            `json:"buf"`
	GoPackages map[string]string `json:"goPackages"`
	Protoc     string            `json:"protoc"`
	Yarn       string            `json:"yarn"`
	ProtobufTs string            `json:"protobuf-ts"`
}

type bufDef struct {
	Go       string `json:"go"`
	Prebuilt string `json:"prebuilt"`
}

func Prebuilt(ctx context.Context, target specs.Platform) (llb.State, error) {
	versions := loadVersions()

	return llbutil.Prebuilt(ctx, versions.Buf.Prebuilt, target)
}

func baseCopies(platform specs.Platform) []llb.StateOption {
	versions := loadVersions()

	golangImage := pins.Image(versions.Buf.Go)

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

	return copies
}

func ImagePlan(platform specs.Platform) (llb.State, error) {
	copies := baseCopies(platform)

	target, err := production.ServerImageLLB(production.StaticBase, platform)
	if err != nil {
		return llb.State{}, nil
	}

	return target.With(copies...), nil
}

func ImagePlanWithNodeJS(platform specs.Platform) llb.State {
	copies := baseCopies(platform)

	baseProtobufTsImage := buildProtobufTsImage(platform)
	protobufTsPaths := [][]string{
		{"protobuf-ts/packages/plugin/bin", "protobuf-ts/packages/plugin/bin"},
		{"protobuf-ts/packages/plugin/build", "protobuf-ts/packages/plugin/build"},
		{"protobuf-ts/packages/plugin/package.json", "protobuf-ts/packages/plugin/package.json"},
		{"protobuf-ts/packages/plugin-framework/build/commonjs", "protobuf-ts/packages/plugin/node_modules/@protobuf-ts/plugin-framework"},
		{"protobuf-ts/packages/runtime/build/commonjs", "protobuf-ts/packages/plugin/node_modules/@protobuf-ts/runtime"},
		{"protobuf-ts/packages/runtime-rpc/build/commonjs", "protobuf-ts/packages/plugin/node_modules/@protobuf-ts/runtime-rpc"},
		{"protobuf-ts/packages/plugin/node_modules/typescript/lib/typescript.js", "protobuf-ts/packages/plugin/node_modules/typescript/index.js"},
	}
	for _, pair := range protobufTsPaths {
		copies = append(copies, llbutil.CopyFrom(baseProtobufTsImage, pair[0], pair[1]))
	}

	target := llbutil.Image(pins.Default("node"), platform)
	return target.With(copies...)
}

func buildProtobufTsImage(platform specs.Platform) llb.State {
	versions := loadVersions()

	alpineImage := pins.Default("alpine")
	target := llbutil.Image(alpineImage, platform)

	// Compiling forked protobuf-ts from sources. Takes ~1 minute.
	// This is temporary, eventually we need to migrate to a published version.
	return target.Run(llb.Shlex("apk add --no-cache git make npm")).Root().
		Run(llb.Shlex("git clone https://github.com/namespacelabs/protobuf-ts.git")).Root().
		// Creating a temporary local branch for the pinned commit
		Run(llb.Shlex(fmt.Sprintf("git checkout -b TmpBranchForPinnedCommit %s", versions.ProtobufTs)),
			llb.Dir("protobuf-ts")).Root().
		Run(llb.Shlex("make npm-install lerna-bootstrap"),
			llb.Dir("protobuf-ts")).Root().
		Run(llb.Shlex("make build"),
			llb.Dir("protobuf-ts/packages/runtime")).Root().
		Run(llb.Shlex("make build"),
			llb.Dir("protobuf-ts/packages/runtime-rpc")).Root().
		Run(llb.Shlex("make build"),
			llb.Dir("protobuf-ts/packages/plugin-framework")).Root().
		Run(llb.Shlex("make build"),
			llb.Dir("protobuf-ts/packages/plugin")).Root().
		Run(llb.Shlex("ls"),
			llb.Dir("protobuf-ts/packages/plugin/bin")).Root()
}

func loadVersions() versionsJSON {
	versionData, err := fs.ReadFile(lib, "versions.json")
	if err != nil {
		panic(err)
	}

	var versions versionsJSON
	if err := json.Unmarshal(versionData, &versions); err != nil {
		panic(err)
	}

	return versions
}

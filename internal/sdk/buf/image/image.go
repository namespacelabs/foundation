// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package image

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
)

type versionsJSON struct {
	Images      map[string]string `json:"images"`
	Buf         bufDef            `json:"buf"`
	GoPackages  map[string]string `json:"goPackages"`
	Protoc      string            `json:"protoc"`
	Yarn        string            `json:"yarn"`
	TsProtocGen string            `json:"ts-protoc-gen"`
	ProtobufTs  string            `json:"protobuf-ts"`
}

type bufDef struct {
	Go       string `json:"go"`
	Prebuilt string `json:"prebuilt"`
}

func Prebuilt(target specs.Platform) llb.State {
	versions := loadVersions()
	return llbutil.Image(versions.Buf.Prebuilt, target)
}

func ImageSource(platform specs.Platform) llb.State {
	versions := loadVersions()

	golangImage := pins.Image(versions.Buf.Go)
	alpineImage := pins.Default("alpine")

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
	target = target.Run(llb.Shlex(fmt.Sprintf("apk add --no-cache protoc=%s", versions.Protoc))).Root()
	target = target.Run(llb.Shlex(fmt.Sprintf("apk add --no-cache yarn=%s", versions.Yarn))).Root()
	target = target.Run(llb.Shlex(fmt.Sprintf("yarn global add ts-protoc-gen@%s", versions.TsProtocGen))).Root()

	// Compiling forked protobuf-ts from sources. Takes ~1 minute.
	// This is temporary, eventually we need to migrate to a published version.
	target = target.Run(llb.Shlex("apk add --no-cache git make npm")).Root().
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

	return target.With(copies...)
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

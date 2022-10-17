// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"embed"
	"encoding/json"
	"io/fs"

	"github.com/moby/buildkit/client/llb"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/internal/dependencies/pins"
	"namespacelabs.dev/foundation/internal/llbutil"
)

var (
	//go:embed versions.json ns-mariadb-entrypoint.sh
	lib embed.FS

	mariadbImage string
	entrypoint   []byte
)

type versionsJSON struct {
	Images  map[string]string `json:"images"`
	MariaDB string            `json:"mariadb"`
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

	mariadbImage = pins.Image(versions.MariaDB)

	entrypoint, err = fs.ReadFile(lib, "ns-mariadb-entrypoint.sh")
	if err != nil {
		panic(err)
	}
}

func makeMariaImageState(platform specs.Platform) llb.State {
	return llbutil.Image(mariadbImage, platform).
		File(llb.Mkfile("ns-mariadb-entrypoint.sh", 0777, entrypoint)).
		File(llb.Rm("/usr/local/bin/docker-entrypoint.sh"))
}

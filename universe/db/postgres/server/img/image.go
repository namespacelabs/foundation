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
	"namespacelabs.dev/foundation/internal/llbutil"
	"namespacelabs.dev/foundation/workspace/pins"
)

var (
	//go:embed versions.json fn-postgres-entrypoint.sh
	lib embed.FS

	postgresImage string
	entrypoint    []byte
)

type versionsJSON struct {
	Images   map[string]string `json:"images"`
	Postgres string            `json:"postgres"`
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

	postgresImage = pins.Image(versions.Postgres)

	entrypoint, err = fs.ReadFile(lib, "fn-postgres-entrypoint.sh")
	if err != nil {
		panic(err)
	}
}

func makePostgresImageState(platform specs.Platform) llb.State {
	target := llbutil.Image(postgresImage, platform)

	return target.File(llb.Mkfile("fn-postgres-entrypoint.sh", 0777, entrypoint))
}

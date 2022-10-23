// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package golang

import (
	"embed"
	"fmt"
	"path/filepath"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/mod/semver"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/sdk/host"
)

var (
	//go:embed versions.json
	lib embed.FS

	v = &host.ParsedVersions{Name: "go", FS: lib}
)

type LocalSDK = host.LocalSDK

func MatchSDK(version string, platform specs.Platform) (compute.Computable[LocalSDK], error) {
	v := v.Get()

	for ver := range v.Versions {
		if semver.Compare("v"+ver, "v"+version) > 0 {
			version = ver
		}
	}

	return SDK(version, platform)
}

func SDK(version string, platform specs.Platform) (compute.Computable[LocalSDK], error) {
	return v.SDK(version, platform, func(ver string, platform specs.Platform) (string, string) {
		return fmt.Sprintf("https://go.dev/dl/go%s.%s-%s.tar.gz", ver, platform.OS, platform.Architecture), "go/bin/go"
	})
}

func GoRoot(sdk LocalSDK) string { return filepath.Join(sdk.Path, "go") }
func GoBin(sdk LocalSDK) string  { return sdk.Binary }

func GoRootEnv(sdk LocalSDK) string {
	return fmt.Sprintf("GOROOT=%s", GoRoot(sdk))
}

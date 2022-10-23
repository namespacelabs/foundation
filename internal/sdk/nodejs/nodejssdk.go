// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package nodejs

import (
	"embed"
	"fmt"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/sdk/host"
)

var (
	//go:embed versions.json
	lib embed.FS

	v = &host.ParsedVersions{Name: "nodejs", FS: lib}
)

type LocalSDK = host.LocalSDK

func SDK(version string, platform specs.Platform) (compute.Computable[LocalSDK], error) {
	return v.SDK(version, platform, func(ver string, platform specs.Platform) (string, string) {
		arch := platform.Architecture
		if arch == "amd64" {
			arch = "x64"
		}

		v := fmt.Sprintf("node-v%s-%s-%s", ver, platform.OS, arch)

		return fmt.Sprintf("https://nodejs.org/dist/v%s/%s.tar.gz", ver, v), fmt.Sprintf("%s/bin/node", v)
	})
}

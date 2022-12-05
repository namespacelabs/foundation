// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package buildkit

import (
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/internal/parsing/platform"
)

func HostPlatform() specs.Platform {
	p := platform.RuntimePlatform()
	p.OS = "linux" // We always run on linux.
	return p
}

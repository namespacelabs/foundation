// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package buildkit

import (
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/internal/parsing/devhost"
)

func HostPlatform() specs.Platform {
	p := devhost.RuntimePlatform()
	p.OS = "linux" // We always run on linux.
	return p
}

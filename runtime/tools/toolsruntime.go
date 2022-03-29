// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tools

import (
	"context"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/runtime/docker"
	"namespacelabs.dev/foundation/runtime/rtypes"
)

type Runtime interface {
	Run(context.Context, rtypes.RunToolOpts) error
	HostPlatform() specs.Platform
}

func Impl() Runtime {
	return docker.Impl()
}
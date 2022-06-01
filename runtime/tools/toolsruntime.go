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

var UseKubernetesRuntime = false

type Runtime interface {
	RunWithOpts(context.Context, rtypes.RunToolOpts, func()) error
	HostPlatform(context.Context) (specs.Platform, error)
}

func Run(ctx context.Context, opts rtypes.RunToolOpts) error {
	return RunWithOpts(ctx, opts, nil)
}

func RunWithOpts(ctx context.Context, opts rtypes.RunToolOpts, onStart func()) error {
	return impl().RunWithOpts(ctx, opts, onStart)
}

func HostPlatform(ctx context.Context) (specs.Platform, error) {
	return impl().HostPlatform(ctx)
}

func impl() Runtime {
	if UseKubernetesRuntime {
		return k8stools{}
	}

	return docker.Impl()
}

// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tools

import (
	"context"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/runtime/docker"
	"namespacelabs.dev/foundation/runtime/rtypes"
	"namespacelabs.dev/foundation/std/cfg"
)

var MakeAlternativeRuntime func(cfg.Configuration) Runtime

type Runtime interface {
	RunWithOpts(context.Context, rtypes.RunToolOpts, func()) error
	HostPlatform(context.Context) (specs.Platform, error)
	CanConsumePublicImages() bool // Whether this runtime implementation can use an ImageID directly if one is available.
}

func Run(ctx context.Context, conf cfg.Configuration, opts rtypes.RunToolOpts) error {
	return RunWithOpts(ctx, conf, opts, nil)
}

func RunWithOpts(ctx context.Context, conf cfg.Configuration, opts rtypes.RunToolOpts, onStart func()) error {
	return impl(conf).RunWithOpts(ctx, opts, onStart)
}

func HostPlatform(ctx context.Context, conf cfg.Configuration) (specs.Platform, error) {
	return impl(conf).HostPlatform(ctx)
}

func CanConsumePublicImages(conf cfg.Configuration) bool {
	return impl(conf).CanConsumePublicImages()
}

func impl(conf cfg.Configuration) Runtime {
	if MakeAlternativeRuntime != nil {
		return MakeAlternativeRuntime(conf)
	}

	return docker.Impl()
}

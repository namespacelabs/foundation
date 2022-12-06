// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package tools

import (
	"context"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/internal/build/buildkit"
	"namespacelabs.dev/foundation/internal/runtime/docker"
	"namespacelabs.dev/foundation/internal/runtime/rtypes"
	"namespacelabs.dev/foundation/std/cfg"
)

var MakeAlternativeRuntime func(cfg.Configuration) Runtime

type Runtime interface {
	RunWithOpts(context.Context, rtypes.RunToolOpts, func()) error
	HostPlatform(context.Context) (specs.Platform, error)
}

func Run(ctx context.Context, conf cfg.Configuration, opts rtypes.RunToolOpts) error {
	return RunWithOpts(ctx, conf, opts, nil)
}

func RunWithOpts(ctx context.Context, conf cfg.Configuration, opts rtypes.RunToolOpts, onStart func()) error {
	return impl(conf).RunWithOpts(ctx, opts, onStart)
}

func HostPlatform(ctx context.Context, conf cfg.Configuration) (specs.Platform, error) {
	if CanUseBuildkit(conf) {
		return buildkit.HostPlatform(), nil
	}

	return impl(conf).HostPlatform(ctx)
}

func impl(conf cfg.Configuration) Runtime {
	if MakeAlternativeRuntime != nil {
		return MakeAlternativeRuntime(conf)
	}

	return docker.Impl()
}

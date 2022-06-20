// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package buf

import (
	"context"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/build/buildkit"
	"namespacelabs.dev/foundation/build/multiplatform"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/sdk/buf/baseimg"
	"namespacelabs.dev/foundation/runtime/tools"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/compute"
)

func Image(ctx context.Context, env ops.Environment, loader workspace.Packages) compute.Computable[oci.Image] {
	platform, err := tools.HostPlatform(ctx)
	if err != nil {
		return compute.Error[oci.Image](err)
	}

	prepared, err := multiplatform.PrepareMultiPlatformImage(ctx, env, build.Plan{
		SourceLabel: "buf.build",
		Spec:        bufBuild{},
		Platforms:   []specs.Platform{platform},
	})
	if err != nil {
		return compute.Error[oci.Image](err)
	}

	return compute.Transform(prepared, func(ctx context.Context, ri oci.ResolvableImage) (oci.Image, error) {
		return ri.Image()
	})
}

type bufBuild struct{}

var _ build.Spec = bufBuild{}

func (bufBuild) BuildImage(ctx context.Context, env ops.Environment, conf build.Configuration) (compute.Computable[oci.Image], error) {
	state := baseimg.MakeBufImageState(*conf.TargetPlatform())

	return buildkit.LLBToImage(ctx, env, conf, state)
}

func (bufBuild) PlatformIndependent() bool { return false }

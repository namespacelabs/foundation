// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package tools

import (
	"context"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/build/binary"
	"namespacelabs.dev/foundation/internal/build/buildkit"
	"namespacelabs.dev/foundation/internal/build/multiplatform"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/planning/tool/protocol"
	"namespacelabs.dev/foundation/internal/runtime/rtypes"
	"namespacelabs.dev/foundation/internal/versions"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/types"
)

func InvokeWithBinary(ctx context.Context, env pkggraph.SealedContext, inv *types.DeferredInvocation, prepared *binary.Prepared) (compute.Computable[*protocol.InvokeResponse], error) {
	c, err := buildkit.Client(ctx, env.Configuration(), nil)
	if err != nil {
		return nil, err
	}

	plan := prepared.Plan
	plan.Platforms = []specs.Platform{c.BuildkitOpts().HostPlatform}

	image, err := multiplatform.PrepareMultiPlatformImage(ctx, env, plan)
	if err != nil {
		return nil, err
	}

	ximage := oci.ResolveImagePlatform(image, c.BuildkitOpts().HostPlatform)

	req := &protocol.ToolRequest{
		ApiVersion:  int32(versions.Builtin().APIVersion),
		ToolPackage: inv.BinaryRef.AsPackageName().String(),
		RequestType: &protocol.ToolRequest_InvokeRequest{
			InvokeRequest: &protocol.InvokeRequest{},
		},
	}

	if inv.GetWithInput().GetTypeUrl() != "" {
		req.Input = append(req.Input, inv.WithInput)
	}

	workingDir := "/"
	if inv.BinaryConfig.WorkingDir != "" {
		workingDir = inv.BinaryConfig.WorkingDir
	}

	run := rtypes.RunBinaryOpts{
		WorkingDir: workingDir,
		Command:    inv.BinaryConfig.Command,
		Args:       inv.BinaryConfig.Args,
		Env:        append(slices.Clone(inv.BinaryConfig.Env), &schema.BinaryConfig_EnvEntry{Name: "HOME", Value: "/tmp"}),
	}

	return compute.Transform("get-response",
		InvokeOnBuildkit[*protocol.ToolResponse](c, "foundation.provision.tool.protocol.InvocationService/Invoke",
			inv.BinaryRef.AsPackageName(), ximage, run, req, LowLevelInvokeOptions{}),
		func(_ context.Context, resp *protocol.ToolResponse) (*protocol.InvokeResponse, error) {
			return resp.InvokeResponse, nil
		}), nil
}

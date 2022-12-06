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
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/foundation/std/types"
)

func InvokeWithBinary(ctx context.Context, env pkggraph.SealedContext, inv *types.DeferredInvocation, prepared *binary.Prepared) (compute.Computable[*protocol.InvokeResponse], error) {
	c, err := buildkit.Client(ctx, env.Configuration(), nil)
	if err != nil {
		return nil, err
	}

	it := &invokeTool{
		conf:       env.Configuration(),
		buildkit:   c,
		invocation: inv,
	}

	plan := prepared.Plan
	plan.Platforms = []specs.Platform{c.BuildkitOpts().HostPlatform}

	image, err := multiplatform.PrepareMultiPlatformImage(ctx, env, plan)
	if err != nil {
		return nil, err
	}

	it.image = compute.Transform("return image", image, func(ctx context.Context, r oci.ResolvableImage) (oci.Image, error) {
		return r.ImageForPlatform(c.BuildkitOpts().HostPlatform)
	})

	return it, nil
}

type invokeTool struct {
	conf       cfg.Configuration       // Does not affect the output.
	buildkit   *buildkit.GatewayClient // Does not affect the output.
	invocation *types.DeferredInvocation
	image      compute.Computable[oci.Image]

	compute.LocalScoped[*protocol.InvokeResponse]
}

func (inv *invokeTool) Action() *tasks.ActionEvent {
	// TODO: support specifying the binary name within the package.
	return tasks.Action("tool.invoke").Arg("package_ref", inv.invocation.BinaryRef.Canonical())
}

func (inv *invokeTool) Inputs() *compute.In {
	in := compute.Inputs().Proto("invocation", inv.invocation)
	return in.Computable("image", inv.image)
}

func (inv *invokeTool) Output() compute.Output {
	return compute.Output{NotCacheable: !inv.invocation.Cacheable}
}

func (inv *invokeTool) Compute(ctx context.Context, r compute.Resolved) (*protocol.InvokeResponse, error) {
	req := &protocol.ToolRequest{
		ApiVersion:  versions.APIVersion,
		ToolPackage: inv.invocation.BinaryRef.AsPackageName().String(),
		RequestType: &protocol.ToolRequest_InvokeRequest{
			InvokeRequest: &protocol.InvokeRequest{},
		},
	}

	if inv.invocation.GetWithInput().GetTypeUrl() != "" {
		req.Input = append(req.Input, inv.invocation.WithInput)
	}

	workingDir := "/"
	if inv.invocation.BinaryConfig.WorkingDir != "" {
		workingDir = inv.invocation.BinaryConfig.WorkingDir
	}

	run := rtypes.RunToolOpts{
		ImageName: inv.invocation.BinaryRef.Canonical(),
		// NoNetworking: true, // XXX security

		RunBinaryOpts: rtypes.RunBinaryOpts{
			WorkingDir: workingDir,
			Command:    inv.invocation.BinaryConfig.Command,
			Args:       inv.invocation.BinaryConfig.Args,
			Env:        append(slices.Clone(inv.invocation.BinaryConfig.Env), &schema.BinaryConfig_EnvEntry{Name: "HOME", Value: "/tmp"}),
		}}

	var invoke LowLevelInvokeOptions[*protocol.ToolRequest, *protocol.ToolResponse]

	run.Image = compute.MustGetDepValue(r, inv.image, "image")

	resp, err := invoke.InvokeOnBuildkit(ctx, inv.buildkit, "foundation.provision.tool.protocol.InvocationService/Invoke", inv.invocation.BinaryRef.AsPackageName(), run.Image, run, req)
	if err != nil {
		return nil, err
	}

	return resp.InvokeResponse, nil
}

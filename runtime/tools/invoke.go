// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tools

import (
	"context"

	"golang.org/x/exp/slices"
	"google.golang.org/grpc"
	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/build/binary"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/provision/tool/protocol"
	"namespacelabs.dev/foundation/runtime/rtypes"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/std/types"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func InvokeWithBinary(ctx context.Context, env planning.Context, inv *types.DeferredInvocation, prepared *binary.Prepared) (compute.Computable[*protocol.InvokeResponse], error) {
	it := &invokeTool{
		env:        env,
		invocation: inv,
	}

	if imgid, ok := build.IsPrebuilt(prepared.Plan.Spec); ok && InvocationCanUseBuildkit {
		it.imageID = imgid
	} else {
		image, err := prepared.Image(ctx, env)
		if err != nil {
			return nil, err
		}
		it.image = compute.Transform(image, func(ctx context.Context, r oci.ResolvableImage) (oci.Image, error) {
			return r.Image()
		})
	}

	return it, nil
}

type invokeTool struct {
	env        planning.Context // Does not affect the output.
	invocation *types.DeferredInvocation
	imageID    oci.ImageID // Use buildkit to invoke instead of the tools runtime.
	image      compute.Computable[oci.Image]

	compute.LocalScoped[*protocol.InvokeResponse]
}

func (inv *invokeTool) Action() *tasks.ActionEvent {
	// TODO: support specifying the binary name within the package.
	return tasks.Action("tool.invoke").Arg("package_ref", inv.invocation.BinaryRef.Canonical())
}

func (inv *invokeTool) Inputs() *compute.In {
	in := compute.Inputs().Proto("invocation", inv.invocation)
	if inv.image != nil {
		return in.Computable("image", inv.image)
	} else {
		return in.JSON("imageID", inv.imageID)
	}
}

func (inv *invokeTool) Output() compute.Output {
	return compute.Output{NotCacheable: !inv.invocation.Cacheable}
}

func (inv *invokeTool) Compute(ctx context.Context, r compute.Resolved) (*protocol.InvokeResponse, error) {
	req := &protocol.ToolRequest{
		ToolPackage: inv.invocation.BinaryRef.AsPackageName().String(),
		RequestType: &protocol.ToolRequest_InvokeRequest{
			InvokeRequest: &protocol.InvokeRequest{},
		},
	}

	if inv.invocation.GetWithInput().GetTypeUrl() != "" {
		req.Input = append(req.Input, inv.invocation.WithInput)
	}

	run := rtypes.RunToolOpts{
		ImageName: inv.invocation.BinaryRef.Canonical(),
		// NoNetworking: true, // XXX security

		RunBinaryOpts: rtypes.RunBinaryOpts{
			WorkingDir: "/",
			Command:    inv.invocation.BinaryConfig.Command,
			Args:       inv.invocation.BinaryConfig.Args,
			Env:        append(slices.Clone(inv.invocation.BinaryConfig.Env), &schema.BinaryConfig_EnvEntry{Name: "HOME", Value: "/tmp"}),
		}}

	var invoke LowLevelInvokeOptions[*protocol.ToolRequest, *protocol.ToolResponse]
	var resp *protocol.ToolResponse
	var err error

	if inv.image != nil {
		run.Image = compute.MustGetDepValue(r, inv.image, "image")

		resp, err = invoke.Invoke(ctx, inv.invocation.BinaryRef.AsPackageName(), run, req, func(conn *grpc.ClientConn) func(context.Context, *protocol.ToolRequest, ...grpc.CallOption) (*protocol.ToolResponse, error) {
			return protocol.NewInvocationServiceClient(conn).Invoke
		})
	} else {
		resp, err = invoke.BuildkitInvocation(ctx, inv.env, "foundation.provision.tool.protocol.InvocationService/Invoke", inv.invocation.BinaryRef.AsPackageName(), inv.imageID, run, req)
	}

	if err != nil {
		return nil, err
	}

	return resp.InvokeResponse, nil
}

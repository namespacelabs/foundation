// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tools

import (
	"context"

	"golang.org/x/exp/slices"
	"google.golang.org/grpc"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/provision/tool/protocol"
	"namespacelabs.dev/foundation/runtime/rtypes"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/types"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func InvokeWithImage(ctx context.Context, env ops.Environment, inv *types.DeferredInvocation, image compute.Computable[oci.Image]) (compute.Computable[*protocol.InvokeResponse], error) {
	return &invokeTool{
		invocation: inv,
		image:      image,
	}, nil
}

type invokeTool struct {
	invocation *types.DeferredInvocation
	image      compute.Computable[oci.Image]

	compute.LocalScoped[*protocol.InvokeResponse]
}

func (inv *invokeTool) Action() *tasks.ActionEvent {
	return tasks.Action("tool.invoke").Arg("package_name", inv.invocation.BinaryPackage)
}

func (inv *invokeTool) Inputs() *compute.In {
	return compute.Inputs().Proto("invocation", inv.invocation).Computable("image", inv.image)
}

func (inv *invokeTool) Output() compute.Output {
	return compute.Output{NotCacheable: !inv.invocation.Cacheable}
}

func (inv *invokeTool) Compute(ctx context.Context, r compute.Resolved) (*protocol.InvokeResponse, error) {
	req := &protocol.ToolRequest{
		ToolPackage: inv.invocation.BinaryPackage,
		RequestType: &protocol.ToolRequest_InvokeRequest{
			InvokeRequest: &protocol.InvokeRequest{},
		},
	}

	if inv.invocation.GetWithInput().GetTypeUrl() != "" {
		req.Input = append(req.Input, inv.invocation.WithInput)
	}

	run := rtypes.RunToolOpts{
		ImageName: inv.invocation.BinaryPackage,
		// NoNetworking: true, // XXX security

		RunBinaryOpts: rtypes.RunBinaryOpts{
			Image:      compute.MustGetDepValue(r, inv.image, "image"),
			WorkingDir: "/",
			Command:    inv.invocation.BinaryConfig.Command,
			Args:       inv.invocation.BinaryConfig.Args,
			Env:        append(slices.Clone(inv.invocation.BinaryConfig.Env), &schema.BinaryConfig_EnvEntry{Name: "HOME", Value: "/tmp"}),
		}}

	invoke := LowLevelInvokeOptions[*protocol.ToolRequest, *protocol.ToolResponse]{}

	resp, err := invoke.Invoke(ctx, schema.PackageName(inv.invocation.BinaryPackage), run, req, func(conn *grpc.ClientConn) func(context.Context, *protocol.ToolRequest, ...grpc.CallOption) (*protocol.ToolResponse, error) {
		return protocol.NewInvocationServiceClient(conn).Invoke
	})
	if err != nil {
		return nil, err
	}

	return resp.InvokeResponse, nil
}

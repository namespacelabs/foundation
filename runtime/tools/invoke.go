// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tools

import (
	"context"

	"google.golang.org/grpc"
	"namespacelabs.dev/foundation/build/binary"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/provision/tool/protocol"
	"namespacelabs.dev/foundation/runtime/rtypes"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/types"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func Invoke(ctx context.Context, env ops.Environment, packages workspace.Packages, inv *types.DeferredInvocation) (compute.Computable[*protocol.InvokeResponse], error) {
	pkg, err := packages.LoadByName(ctx, schema.PackageName(inv.Binary))
	if err != nil {
		return nil, err
	}

	target := Impl().HostPlatform()
	prepared, err := binary.PlanImage(ctx, pkg, env, true, &target)
	if err != nil {
		return nil, err
	}

	return &invokeTool{
		invocation: inv,
		prepared:   prepared,
	}, nil
}

type invokeTool struct {
	invocation *types.DeferredInvocation
	prepared   *binary.PreparedImage

	compute.LocalScoped[*protocol.InvokeResponse]
}

func (inv *invokeTool) Action() *tasks.ActionEvent {
	return tasks.Action("tool.invoke").Arg("package_name", inv.invocation.Binary)
}

func (inv *invokeTool) Inputs() *compute.In {
	return compute.Inputs().Str("name", inv.prepared.Name).Strs("command", inv.prepared.Command).Computable("image", inv.prepared.Image).Proto("invocation", inv.invocation)
}

func (inv *invokeTool) Output() compute.Output {
	return compute.Output{NotCacheable: !inv.invocation.Cacheable}
}

func (inv *invokeTool) Compute(ctx context.Context, r compute.Resolved) (*protocol.InvokeResponse, error) {
	req := &protocol.ToolRequest{
		ToolPackage: inv.invocation.Binary,
		RequestType: &protocol.ToolRequest_InvokeRequest{
			InvokeRequest: &protocol.InvokeRequest{},
		},
	}

	if inv.invocation.GetWithInput().GetTypeUrl() != "" {
		req.Input = append(req.Input, inv.invocation.WithInput)
	}

	run := rtypes.RunToolOpts{
		ImageName: inv.invocation.Binary,
		// NoNetworking: true, // XXX security

		RunBinaryOpts: rtypes.RunBinaryOpts{
			Image:      compute.MustGetDepValue(r, inv.prepared.Image, "image"),
			WorkingDir: "/",
			Command:    inv.prepared.Command,
			Env:        map[string]string{"HOME": "/tmp"},
			RunAsUser:  true,
		}}

	invoke := LowLevelInvokeOptions[*protocol.ToolRequest, *protocol.ToolResponse]{}

	resp, err := invoke.Invoke(ctx, schema.PackageName(inv.invocation.Binary), run, req, func(conn *grpc.ClientConn) func(context.Context, *protocol.ToolRequest, ...grpc.CallOption) (*protocol.ToolResponse, error) {
		return protocol.NewInvocationServiceClient(conn).Invoke
	})
	if err != nil {
		return nil, err
	}

	return resp.InvokeResponse, nil
}

// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tools

import (
	"bytes"
	"context"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/build/binary"
	"namespacelabs.dev/foundation/internal/console"
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
	var out bytes.Buffer

	req := &protocol.ToolRequest{
		ToolPackage: inv.invocation.Binary,
		RequestType: &protocol.ToolRequest_InvokeRequest{
			InvokeRequest: &protocol.InvokeRequest{},
		},
	}

	reqbytes, err := proto.Marshal(req)
	if err != nil {
		return nil, err
	}

	run := rtypes.RunToolOpts{
		ImageName: inv.invocation.Binary,
		IO: rtypes.IO{
			Stdin:  bytes.NewReader(reqbytes),
			Stdout: &out,
			Stderr: console.Output(ctx, inv.prepared.Name),
		},
		// NoNetworking: true, // XXX security

		RunBinaryOpts: rtypes.RunBinaryOpts{
			Image:      compute.GetDepValue(r, inv.prepared.Image, "image"),
			WorkingDir: "/",
			Command:    inv.prepared.Command,
			Env:        map[string]string{"HOME": "/tmp"},
			RunAsUser:  true,
		}}

	if err := Impl().Run(ctx, run); err != nil {
		return nil, err
	}

	resp := &protocol.ToolResponse{}
	if err := proto.Unmarshal(out.Bytes(), resp); err != nil {
		return nil, err
	}

	return resp.InvokeResponse, nil
}

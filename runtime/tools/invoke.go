// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tools

import (
	"bytes"
	"context"

	"namespacelabs.dev/foundation/build/binary"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/runtime/rtypes"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

type InvocationResult struct {
	Bytes []byte
}

func Invoke(ctx context.Context, env ops.Environment, packages workspace.Packages, binpkg schema.PackageName, cacheable bool) (compute.Computable[InvocationResult], error) {
	pkg, err := packages.LoadByName(ctx, binpkg)
	if err != nil {
		return nil, err
	}

	target := Impl().HostPlatform()
	prepared, err := binary.PlanImage(ctx, pkg, env, true, &target)
	if err != nil {
		return nil, err
	}

	return &invokeTool{
		pkg:       binpkg,
		prepared:  prepared,
		cacheable: cacheable,
	}, nil
}

type invokeTool struct {
	pkg       schema.PackageName
	prepared  *binary.PreparedImage
	cacheable bool

	compute.LocalScoped[InvocationResult]
}

func (inv *invokeTool) Action() *tasks.ActionEvent {
	return tasks.Action("tool.invoke").Arg("package_name", inv.pkg)
}

func (inv *invokeTool) Inputs() *compute.In {
	return compute.Inputs().Str("name", inv.prepared.Name).Strs("command", inv.prepared.Command).Computable("image", inv.prepared.Image).Stringer("pkg", inv.pkg)
}

func (inv *invokeTool) Output() compute.Output { return compute.Output{NotCacheable: !inv.cacheable} }

func (inv *invokeTool) Compute(ctx context.Context, r compute.Resolved) (InvocationResult, error) {
	var out bytes.Buffer

	run := rtypes.RunToolOpts{
		ImageName: inv.pkg.String(),
		IO: rtypes.IO{
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
		return InvocationResult{}, err
	}

	return InvocationResult{Bytes: out.Bytes()}, nil
}

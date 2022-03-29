// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tool

import (
	"bytes"
	"context"
	"io/fs"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/provision/tool/protocol"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/rtypes"
	"namespacelabs.dev/foundation/runtime/tools"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func Invoke(ctx context.Context, env ops.Environment, r *Handler, stack *schema.Stack, focus schema.PackageName, props *rtypes.ProvisionProps, ev protocol.Lifecycle) compute.Computable[*protocol.ToolResponse] {
	return &cacheableInvocation{
		handler:   *r,
		stack:     stack,
		focus:     focus,
		env:       env.Proto(),
		props:     props,
		lifecycle: ev,
	}
}

type cacheableInvocation struct {
	handler   Handler
	stack     *schema.Stack
	focus     schema.PackageName
	env       *schema.Environment
	props     *rtypes.ProvisionProps
	lifecycle protocol.Lifecycle

	compute.LocalScoped[*protocol.ToolResponse]
}

func (inv *cacheableInvocation) Action() *tasks.ActionEvent {
	return tasks.Task(runtime.TaskProvisionInvoke).Scope(inv.handler.Source.PackageName)
}

func (inv *cacheableInvocation) Inputs() *compute.In {
	invocation := *inv.handler.Invocation
	invocation.Image = nil // To make the invocation JSON serializable.
	return compute.Inputs().
		Computable("image", inv.handler.Invocation.Image).
		JSON("handler", Handler{For: inv.handler.For, Source: inv.handler.Source, Invocation: &invocation}). // Without image and PackageAbsPath.
		Proto("stack", inv.stack).Stringer("focus", inv.focus).Proto("env", inv.env).
		Proto("props", inv.props).JSON("lifecycle", inv.lifecycle)
}

func (inv *cacheableInvocation) Output() compute.Output {
	// To make invocations cacheable we need to enumerate the contents of the mounts.
	mountCount := len(inv.handler.Invocation.Mounts) + len(inv.props.LocalMapping)

	return compute.Output{
		NotCacheable:     inv.handler.Invocation.NoCache,
		NonDeterministic: mountCount > 0,
	}
}

func (inv *cacheableInvocation) Compute(ctx context.Context, deps compute.Resolved) (*protocol.ToolResponse, error) {
	resolvedImage := compute.GetDepValue(deps, inv.handler.Invocation.Image, "image")
	r := inv.handler

	req := &protocol.ToolRequest{
		Stack:         inv.stack,
		FocusedServer: inv.focus.String(),
		ToolPackage:   r.Source.PackageName.String(),
		Env:           inv.env,
	}

	switch inv.lifecycle {
	case protocol.Lifecycle_PROVISION:
		req.RequestType = &protocol.ToolRequest_ApplyRequest{ApplyRequest: &protocol.ApplyRequest{}}
	case protocol.Lifecycle_SHUTDOWN:
		req.RequestType = &protocol.ToolRequest_DeleteRequest{DeleteRequest: &protocol.DeleteRequest{}}
	default:
		return nil, fnerrors.InternalError("%v: no support for lifecycle", inv.lifecycle)
	}

	// Snapshots are pushed synchrously with the invocation itself. This is bound
	// to become a source of latency, as we're not pipelining the starting of the
	// execution with the making the snapshot contents available to the tool. It
	// will need revisiting.
	for _, snapshot := range r.Invocation.Snapshots {
		snap := &protocol.Snapshot{Name: snapshot.Name}

		if err := fnfs.VisitFiles(ctx, snapshot.Contents, func(path string, contents []byte, _ fs.DirEntry) error {
			snap.Entry = append(snap.Entry, &protocol.Snapshot_FileEntry{
				Path:     path,
				Contents: contents,
			})
			return nil
		}); err != nil {
			return nil, fnerrors.TransientError("%s: computing snapshot failed: %w", snapshot.Name, err)
		}

		req.Snapshot = append(req.Snapshot, snap)
	}

	opts := rtypes.RunToolOpts{
		ImageName: r.Invocation.ImageName,
		RunBinaryOpts: rtypes.RunBinaryOpts{
			Image:      resolvedImage,
			Command:    r.Invocation.Command,
			Args:       rtypes.FlattenArgs(r.Invocation.Args),
			WorkingDir: r.Invocation.WorkingDir,
		},
		MountAbsRoot: inv.handler.PackageAbsPath,
		// Don't let an invocation reach out, it should be hermetic. Tools are
		// expected to produce operations which can be inspected. And then these
		// operations are applied by the caller.
		NoNetworking: true,
	}

	opts.Mounts = append(opts.Mounts, r.Invocation.Mounts...)
	opts.Mounts = append(opts.Mounts, inv.props.GetLocalMapping()...)
	req.Input = append(req.Input, inv.props.GetProvisionInput()...)

	// XXX security: think through whether it is OK or not to expose Snapshots here.
	// For now, assume not.
	attachments := tasks.Attachments(ctx)
	if attachments.IsRecording() {
		reqcopy := proto.Clone(req).(*protocol.ToolRequest)
		reqcopy.Snapshot = nil
		attachments.AttachSerializable("request.textpb", "", reqcopy)
	}

	reqBytes, err := proto.Marshal(req)
	if err != nil {
		return nil, err
	}

	var out bytes.Buffer

	opts.Stdin = bytes.NewReader(reqBytes)
	opts.Stdout = &out
	opts.Stderr = console.Output(ctx, "tool")

	if err := tools.Impl().Run(ctx, opts); err != nil {
		return nil, fnerrors.InternalError("%s: prepare failed: %w", r.Source, err)
	}

	resp := &protocol.ToolResponse{}
	if err := proto.Unmarshal(out.Bytes(), resp); err != nil {
		return nil, fnerrors.InternalError("failed to parse tool output: %w", err)
	}

	attachments.AttachSerializable("response.textpb", "", resp)

	return resp, nil
}
// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tool

import (
	"context"
	"fmt"
	"io/fs"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/dustin/go-humanize"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/bytestream"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/provision/tool/protocol"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/rtypes"
	"namespacelabs.dev/foundation/runtime/tools"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

var InvocationDebug = false

const toolBackoff = 500 * time.Millisecond

type InvokeProps struct {
	Event          protocol.Lifecycle
	ProvisionInput []*anypb.Any
}

func MakeInvocation(ctx context.Context, env ops.Environment, r *Definition, stack *schema.Stack, focus schema.PackageName, props InvokeProps) compute.Computable[*protocol.ToolResponse] {
	return &cacheableInvocation{
		handler: *r,
		stack:   stack,
		focus:   focus,
		env:     env.Proto(),
		props:   props,
	}
}

type cacheableInvocation struct {
	handler Definition
	stack   *schema.Stack
	focus   schema.PackageName
	env     *schema.Environment
	props   InvokeProps

	compute.LocalScoped[*protocol.ToolResponse]
}

func (inv *cacheableInvocation) Action() *tasks.ActionEvent {
	return tasks.Action(runtime.TaskProvisionInvoke).
		Scope(inv.handler.Source.PackageName).
		Arg("target", inv.handler.TargetServer)
}

func (inv *cacheableInvocation) Inputs() *compute.In {
	invocation := *inv.handler.Invocation // Copy
	invocation.Image = nil                // To make the invocation JSON serializable.
	invocation.PublicImageID = nil

	in := compute.Inputs().
		JSON("handler", Definition{TargetServer: inv.handler.TargetServer, Source: inv.handler.Source, Invocation: &invocation}). // Without image and PackageAbsPath.
		Proto("stack", inv.stack).Stringer("focus", inv.focus).Proto("env", inv.env).
		JSON("props", inv.props)

	if tools.CanConsumePublicImages() && inv.handler.Invocation.PublicImageID != nil {
		return in.JSON("publicImageID", *inv.handler.Invocation.PublicImageID)
	} else {
		return in.Computable("image", inv.handler.Invocation.Image)
	}
}

func (inv *cacheableInvocation) Output() compute.Output {
	return compute.Output{
		NotCacheable: inv.handler.Invocation.NoCache,
	}
}

func (inv *cacheableInvocation) Compute(ctx context.Context, deps compute.Resolved) (res *protocol.ToolResponse, err error) {
	r := inv.handler

	req := &protocol.ToolRequest{
		ToolPackage: r.Source.PackageName.String(),
		// XXX temporary.
		Stack:         inv.stack,
		FocusedServer: inv.focus.String(),
		Env:           inv.env,
	}

	header := &protocol.StackRelated{
		Stack:         inv.stack,
		FocusedServer: inv.focus.String(),
		Env:           inv.env,
	}

	switch inv.props.Event {
	case protocol.Lifecycle_PROVISION:
		req.RequestType = &protocol.ToolRequest_ApplyRequest{
			ApplyRequest: &protocol.ApplyRequest{
				Header: header,
			}}
	case protocol.Lifecycle_SHUTDOWN:
		req.RequestType = &protocol.ToolRequest_DeleteRequest{
			DeleteRequest: &protocol.DeleteRequest{
				Header: header,
			}}
	default:
		return nil, fnerrors.InternalError("%v: no support for lifecycle", inv.props.Event)
	}

	// Snapshots are pushed synchrously with the invocation itself. This is bound
	// to become a source of latency, as we're not pipelining the starting of the
	// execution with the making the snapshot contents available to the tool. It
	// will need revisiting.
	for _, snapshot := range r.Invocation.Snapshots {
		snap := &protocol.Snapshot{Name: snapshot.Name}

		if err := fnfs.VisitFiles(ctx, snapshot.Contents, func(path string, blob bytestream.ByteStream, _ fs.DirEntry) error {
			contents, err := bytestream.ReadAll(blob)
			if err != nil {
				return err
			}

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
			Command:    r.Invocation.Command,
			Args:       r.Invocation.Args,
			WorkingDir: r.Invocation.WorkingDir,
		},
		// Don't let an invocation reach out, it should be hermetic. Tools are
		// expected to produce operations which can be inspected. And then these
		// operations are applied by the caller.
		NoNetworking: true,
	}

	if tools.CanConsumePublicImages() && r.Invocation.PublicImageID != nil {
		opts.PublicImageID = r.Invocation.PublicImageID
	} else {
		opts.Image = compute.MustGetDepValue(deps, inv.handler.Invocation.Image, "image")
	}

	if InvocationDebug {
		opts.RunBinaryOpts.Args = append(opts.RunBinaryOpts.Args, "--debug")
	}

	req.Input = append(req.Input, inv.props.ProvisionInput...)

	for _, inject := range r.Invocation.Inject {
		provider, ok := registrations[inject.Type]
		if !ok {
			return nil, fnerrors.BadInputError("%s: no such provider", inject)
		}

		input, err := provider(ctx, inv.env, inv.stack.GetServer(inv.focus))
		if err != nil {
			return nil, err
		}

		req.Input = append(req.Input, input)
	}

	count := 0
	err = backoff.Retry(func() error {
		count++

		res, err = tools.LowLevelInvokeOptions[*protocol.ToolRequest, *protocol.ToolResponse]{
			RedactRequest: func(req proto.Message) proto.Message {
				// XXX security: think through whether it is OK or not to expose Snapshots here.
				// For now, assume not.
				reqcopy := protos.Clone(req).(*protocol.ToolRequest)
				reqcopy.Snapshot = nil
				return reqcopy
			},
		}.Invoke(ctx, r.Source.PackageName, opts, req, func(conn *grpc.ClientConn) func(context.Context, *protocol.ToolRequest, ...grpc.CallOption) (*protocol.ToolResponse, error) {
			return protocol.NewInvocationServiceClient(conn).Invoke
		})

		if fnerrors.IsInvocationError(err) {
			fmt.Fprintf(console.Stderr(ctx), "%s: Invoking provisioning tool (%s try) encountered transient failure: %v. Will retry in %v.\n", r.Source.PackageName, humanize.Ordinal(count), err, toolBackoff)

			return err
		}

		// Only retry invocation errors
		return backoff.Permanent(err)
	}, backoff.WithContext(backoff.NewConstantBackOff(toolBackoff), ctx))

	return res, err
}

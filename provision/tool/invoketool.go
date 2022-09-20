// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tool

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/dustin/go-humanize"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/engine/compute"
	"namespacelabs.dev/foundation/internal/bytestream"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/internal/versions"
	"namespacelabs.dev/foundation/provision/tool/protocol"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/rtypes"
	"namespacelabs.dev/foundation/runtime/tools"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace/tasks"
)

var InvocationDebug = false

const toolBackoff = 500 * time.Millisecond

type InvokeProps struct {
	Event          protocol.Lifecycle
	ProvisionInput []*anypb.Any
}

func MakeInvocation(ctx context.Context, env planning.Context, planner runtime.Planner, r *Definition, stack *schema.Stack, focus schema.PackageName, props InvokeProps) (compute.Computable[*protocol.ToolResponse], error) {
	// Calculate injections early on to make sure that they're part of the cache key.
	var injections []*anypb.Any
	for _, inject := range r.Invocation.Inject {
		provider, ok := registrations[inject.Type]
		if !ok {
			return nil, fnerrors.BadInputError("%s: no such provider", inject)
		}

		input, err := provider(ctx, env, planner, stack.GetServer(focus))
		if err != nil {
			return nil, err
		}

		injections = append(injections, input)
	}

	return &cacheableInvocation{
		handler:    *r,
		stack:      stack,
		focus:      focus,
		env:        env,
		props:      props,
		injections: injections,
	}, nil
}

type cacheableInvocation struct {
	env        planning.Context // env.Proto() is used as cache key.
	handler    Definition
	stack      *schema.Stack
	focus      schema.PackageName
	props      InvokeProps
	injections []*anypb.Any

	compute.LocalScoped[*protocol.ToolResponse]
}

func (inv *cacheableInvocation) Action() *tasks.ActionEvent {
	return tasks.Action("provision.invoke").
		Scope(inv.handler.Source.PackageName).
		Arg("target", inv.handler.TargetServer)
}

func (inv *cacheableInvocation) Inputs() *compute.In {
	invocation := *inv.handler.Invocation // Copy
	invocation.Image = nil                // To make the invocation JSON serializable.
	invocation.PublicImageID = nil

	in := compute.Inputs().
		JSON("handler", Definition{TargetServer: inv.handler.TargetServer, Source: inv.handler.Source, Invocation: &invocation}). // Without image and PackageAbsPath.
		Proto("stack", inv.stack).
		Stringer("focus", inv.focus).
		Proto("env", inv.env.Environment()).
		JSON("props", inv.props).
		JSON("injections", inv.injections)

	if (tools.InvocationCanUseBuildkit || tools.CanConsumePublicImages(inv.env.Configuration())) && inv.handler.Invocation.PublicImageID != nil {
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
		ApiVersion:  versions.APIVersion,
		ToolPackage: r.Source.PackageName.String(),
		// XXX temporary.
		Stack:         inv.stack,
		FocusedServer: inv.focus.String(),
		Env:           inv.env.Environment(),
	}

	header := &protocol.StackRelated{
		Stack:         inv.stack,
		FocusedServer: inv.focus.String(),
		Env:           inv.env.Environment(),
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

	if (tools.InvocationCanUseBuildkit || tools.CanConsumePublicImages(inv.env.Configuration())) && r.Invocation.PublicImageID != nil {
		opts.PublicImageID = r.Invocation.PublicImageID
	} else {
		opts.Image = compute.MustGetDepValue(deps, inv.handler.Invocation.Image, "image")
	}

	opts.SupportedToolVersion = r.Invocation.SupportedToolVersion

	if InvocationDebug {
		opts.RunBinaryOpts.Args = append(opts.RunBinaryOpts.Args, "--debug")
	}

	req.Input = append(req.Input, inv.props.ProvisionInput...)
	req.Input = append(req.Input, inv.injections...)

	invocation := tools.LowLevelInvokeOptions[*protocol.ToolRequest, *protocol.ToolResponse]{RedactRequest: redactMessage}

	if tools.InvocationCanUseBuildkit && opts.PublicImageID != nil {
		return invocation.InvokeOnBuildkit(ctx, inv.env.Configuration(), "foundation.provision.tool.protocol.InvocationService/Invoke",
			r.Source.PackageName, *r.Invocation.PublicImageID, opts, req)
	}

	count := 0
	err = backoff.Retry(func() error {
		count++

		res, err = invocation.Invoke(ctx, inv.env.Configuration(), r.Source.PackageName, opts, req, func(conn *grpc.ClientConn) func(context.Context, *protocol.ToolRequest, ...grpc.CallOption) (*protocol.ToolResponse, error) {
			return protocol.NewInvocationServiceClient(conn).Invoke
		})

		if errors.Is(err, &fnerrors.InvocationErr{}) {
			fmt.Fprintf(console.Stderr(ctx), "%s: Invoking provisioning tool (%s try) encountered transient failure: %v. Will retry in %v.\n", r.Source.PackageName, humanize.Ordinal(count), err, toolBackoff)

			return err
		}

		// Only retry invocation errors
		return backoff.Permanent(err)
	}, backoff.WithContext(backoff.NewConstantBackOff(toolBackoff), ctx))

	return res, err
}

func redactMessage(req proto.Message) proto.Message {
	// XXX security: think through whether it is OK or not to expose Snapshots here.
	// For now, assume not.
	reqcopy := protos.Clone(req).(*protocol.ToolRequest)
	reqcopy.Snapshot = nil
	return reqcopy
}

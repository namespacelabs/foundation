// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package tool

import (
	"context"
	"io/fs"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/bytestream"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/planning/invocation"
	"namespacelabs.dev/foundation/internal/planning/tool/protocol"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/internal/runtime/rtypes"
	"namespacelabs.dev/foundation/internal/runtime/tools"
	"namespacelabs.dev/foundation/internal/versions"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/tasks"
)

var InvocationDebug = false

const toolBackoff = 500 * time.Millisecond

type InvokeProps struct {
	Event          protocol.Lifecycle
	ProvisionInput []*anypb.Any
}

func MakeInvocation(ctx context.Context, env cfg.Context, planner runtime.Planner, r *Definition, stack *schema.Stack, focus schema.PackageName, props InvokeProps) (compute.Computable[*protocol.ToolResponse], error) {
	x := makeBaseInvocation(ctx, env, r, props)
	// Calculate injections early on to make sure that they're part of the cache key.
	for _, inject := range r.Invocation.Inject {
		provider, ok := registrations[inject.Type]
		if !ok {
			return nil, fnerrors.BadInputError("%s: no such provider", inject)
		}

		if stack == nil {
			return nil, fnerrors.InternalError("no stack was provided")
		}

		input, err := provider(ctx, env, planner, stack.GetServer(focus))
		if err != nil {
			return nil, err
		}

		x.injections = append(x.injections, input)
	}

	x.stack = stack
	x.focus = focus

	return x, nil
}

func MakeInvocationNoInjections(ctx context.Context, env cfg.Context, r *Definition, props InvokeProps) (compute.Computable[*protocol.ToolResponse], error) {
	if len(r.Invocation.Inject) > 0 {
		return nil, fnerrors.InternalError("injections are not supported in this path")
	}

	return makeBaseInvocation(ctx, env, r, props), nil
}

// The caller must ensure that injections are handled.
func makeBaseInvocation(ctx context.Context, env cfg.Context, r *Definition, props InvokeProps) *cacheableInvocation {
	return &cacheableInvocation{
		targetServer: r.TargetServer,
		source:       r.Source,
		invocation:   r.Invocation,
		env:          env,
		props:        props,
	}
}

type cacheableInvocation struct {
	env          cfg.Context        // env.Proto() is used as cache key.
	targetServer schema.PackageName // May be unspecified. For logging purposes only.
	source       Source             // Where the invocation was declared.
	invocation   *invocation.Invocation
	stack        *schema.Stack
	focus        schema.PackageName
	props        InvokeProps
	injections   []*anypb.Any

	compute.LocalScoped[*protocol.ToolResponse]
}

func (inv *cacheableInvocation) Action() *tasks.ActionEvent {
	action := tasks.Action("provision.invoke").
		Scope(inv.source.PackageName)

	if inv.targetServer != "" {
		return action.Arg("target", inv.targetServer)
	}

	return action
}

func (inv *cacheableInvocation) Inputs() *compute.In {
	invocation := *inv.invocation // Copy
	invocation.Image = nil        // To make the invocation JSON serializable.
	invocation.Buildkit = nil     // To make the invocation JSON serializable.

	return compute.Inputs().
		JSON("invocation", invocation). // Without image and PackageAbsPath.
		JSON("source", inv.source).
		Proto("stack", inv.stack).
		Stringer("focus", inv.focus).
		Proto("env", inv.env.Environment()).
		JSON("props", inv.props).
		JSON("injections", inv.injections).
		Computable("image", inv.invocation.Image)
}

func (inv *cacheableInvocation) Output() compute.Output {
	return compute.Output{
		NotCacheable: inv.invocation.NoCache,
	}
}

func (inv *cacheableInvocation) Compute(ctx context.Context, deps compute.Resolved) (res *protocol.ToolResponse, err error) {
	invocation := inv.invocation

	req := &protocol.ToolRequest{
		ApiVersion:  versions.APIVersion,
		ToolPackage: inv.source.PackageName.String(),
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
	for _, snapshot := range invocation.Snapshots {
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
		ImageName: invocation.ImageName,
		RunBinaryOpts: rtypes.RunBinaryOpts{
			Command:    invocation.Command,
			Args:       invocation.Args,
			WorkingDir: invocation.WorkingDir,
		},
		// Don't let an invocation reach out, it should be hermetic. Tools are
		// expected to produce operations which can be inspected. And then these
		// operations are applied by the caller.
		NoNetworking: true,
	}

	resolvable := compute.MustGetDepValue(deps, invocation.Image, "image")

	cli, err := inv.invocation.Buildkit.MakeClient(ctx)
	if err != nil {
		return nil, err
	}

	if image, err := resolvable.ImageForPlatform(cli.BuildkitOpts().HostPlatform); err == nil {
		opts.Image = image
	} else {
		return nil, err
	}

	opts.SupportedToolVersion = invocation.SupportedToolVersion

	if InvocationDebug {
		opts.RunBinaryOpts.Args = append(opts.RunBinaryOpts.Args, "--debug")
	}

	req.Input = append(req.Input, inv.props.ProvisionInput...)
	req.Input = append(req.Input, inv.injections...)

	x := tools.LowLevelInvokeOptions[*protocol.ToolRequest, *protocol.ToolResponse]{RedactRequest: redactMessage}

	return x.InvokeOnBuildkit(ctx, cli, "foundation.provision.tool.protocol.InvocationService/Invoke",
		inv.source.PackageName, opts.Image, opts, req)
}

func redactMessage(req proto.Message) proto.Message {
	// XXX security: think through whether it is OK or not to expose Snapshots here.
	// For now, assume not.
	reqcopy := protos.Clone(req).(*protocol.ToolRequest)
	reqcopy.Snapshot = nil
	return reqcopy
}

// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package tool

import (
	"context"
	"io/fs"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/bytestream"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/planning/tool/protocol"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/internal/runtime/rtypes"
	"namespacelabs.dev/foundation/internal/runtime/tools"
	"namespacelabs.dev/foundation/internal/secrets"
	"namespacelabs.dev/foundation/internal/versions"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
)

var InvocationDebug = false

type InvokeProps struct {
	Event          protocol.Lifecycle
	ProvisionInput []*anypb.Any
}

func MakeInvocation(ctx context.Context, env cfg.Context, planner runtime.Planner, r *Definition, stack *schema.Stack, focus schema.PackageName, props InvokeProps) (compute.Computable[*protocol.ToolResponse], error) {
	// Calculate injections early on to make sure that they're part of the cache key.
	var injections []*anypb.Any
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

		injections = append(injections, input)
	}

	return makeBaseInvocation(ctx, env, nil, r, props, &protocol.StackRelated{
		Env:           env.Environment(),
		Stack:         stack,
		FocusedServer: focus.String(),
	}, injections)
}

func MakeInvocationNoInjections(ctx context.Context, env cfg.Context, secrets secrets.GroundedSecrets, r *Definition, props InvokeProps) (compute.Computable[*protocol.ToolResponse], error) {
	if len(r.Invocation.Inject) > 0 {
		return nil, fnerrors.InternalError("injections are not supported in this path")
	}

	return makeBaseInvocation(ctx, env, secrets, r, props, nil, nil)
}

// The caller must ensure that injections are handled.
func makeBaseInvocation(ctx context.Context, env cfg.Context, secrets secrets.GroundedSecrets, r *Definition, props InvokeProps, header *protocol.StackRelated, injections []*anypb.Any) (compute.Computable[*protocol.ToolResponse], error) {
	invocation := r.Invocation

	req := &protocol.ToolRequest{
		ApiVersion:  int32(versions.Builtin().APIVersion),
		ToolPackage: r.Source.PackageName.String(),
		// XXX temporary.
		Stack:         header.GetStack(),
		FocusedServer: header.GetFocusedServer(),
		Env:           env.Environment(),
	}

	switch props.Event {
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
		return nil, fnerrors.InternalError("%v: no support for lifecycle", props.Event)
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

	opts := rtypes.RunBinaryOpts{
		Command:    invocation.Config.Command,
		Args:       invocation.Config.Args,
		WorkingDir: invocation.Config.WorkingDir,
		Env:        invocation.Config.Env,
	}

	cli, err := invocation.Buildkit.MakeClient(ctx)
	if err != nil {
		return nil, err
	}

	ximage := oci.ResolveImagePlatform(invocation.Image, cli.BuildkitOpts().HostPlatform)

	if InvocationDebug {
		opts.Args = append(opts.Args, "--debug")
	}

	req.Input = append(req.Input, props.ProvisionInput...)
	req.Input = append(req.Input, injections...)

	return tools.InvokeOnBuildkit[*protocol.ToolResponse](cli, secrets,
		"foundation.provision.tool.protocol.InvocationService/Invoke",
		r.Source.PackageName, ximage, opts, req,
		tools.LowLevelInvokeOptions{RedactRequest: redactMessage}), nil
}

func redactMessage(req proto.Message) proto.Message {
	// XXX security: think through whether it is OK or not to expose Snapshots here.
	// For now, assume not.
	reqcopy := protos.Clone(req).(*protocol.ToolRequest)
	reqcopy.Snapshot = nil
	return reqcopy
}

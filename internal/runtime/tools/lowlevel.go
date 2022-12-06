// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package tools

import (
	"context"
	"fmt"
	"io/fs"

	"github.com/moby/buildkit/client/llb"
	"github.com/opencontainers/go-digest"
	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/build"
	"namespacelabs.dev/foundation/internal/build/buildkit"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/internal/runtime/rtypes"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/tasks"
)

type LowLevelInvokeOptions[Req, Resp proto.Message] struct {
	RedactRequest  func(proto.Message) proto.Message
	RedactResponse func(proto.Message) proto.Message
}

func attachToAction(ctx context.Context, name string, msg proto.Message, redactMessage func(proto.Message) proto.Message) {
	attachments := tasks.Attachments(ctx)
	err := attachments.AttachSerializable(name+".textpb", "", redact(msg, redactMessage))
	if err != nil {
		fmt.Fprintf(console.Debug(ctx), "failed to serialize request: %v", err)
	}
}

func (oo LowLevelInvokeOptions[Req, Resp]) InvokeOnBuildkit(ctx context.Context, c *buildkit.GatewayClient, method string, pkg schema.PackageName, image oci.Image, opts rtypes.RunToolOpts, req Req) (Resp, error) {
	return tasks.Return(ctx, tasks.Action("buildkit.invocation").Scope(pkg).Arg("method", method).LogLevel(1), func(ctx context.Context) (Resp, error) {
		attachToAction(ctx, "request", req, oo.RedactRequest)

		resp := protos.NewFromType[Resp]()

		image, err := oci.EnsureCached(ctx, image)
		if err != nil {
			return resp, err
		}

		d, err := image.Digest()
		if err != nil {
			return resp, err
		}

		tasks.Attachments(ctx).AddResult("ref", d.String())

		if !c.BuildkitOpts().SupportsCanonicalBuilds {
			return resp, fnerrors.InvocationError("buildkit", "the target buildkit does not have the required capabilities (ocilayout input), please upgrade")
		}

		p := c.BuildkitOpts().HostPlatform

		base := llb.OCILayout("cache", digest.Digest(d.String()), llb.WithCustomNamef("%s: base image (%s)", pkg, d))

		args := append(slices.Clone(opts.Command), opts.Args...)
		args = append(args, "--inline_invocation="+method)
		args = append(args, "--inline_invocation_input=/request/request.binarypb")
		args = append(args, "--inline_invocation_output=/out/response.binarypb")

		runOpts := []llb.RunOption{llb.ReadonlyRootFS(), llb.Network(llb.NetModeNone), llb.Args(args)}
		if opts.WorkingDir != "" {
			runOpts = append(runOpts, llb.Dir(opts.WorkingDir))
		}

		run := base.Run(runOpts...)

		requestBytes, err := proto.Marshal(req)
		if err != nil {
			return resp, err
		}

		requestState := llb.Scratch().File(llb.Mkfile("request.binarypb", 0644, requestBytes))

		run.AddMount("/request", requestState, llb.Readonly)
		out := run.AddMount("/out", llb.Scratch())

		output, err := buildkit.BuildFilesystem(ctx, c, build.NewBuildTarget(&p).WithSourceLabel("Invocation %s", pkg).WithSourcePackage(pkg), out)
		if err != nil {
			return resp, err
		}

		fsys, err := compute.GetValue(ctx, output)
		if err != nil {
			return resp, err
		}

		responseBytes, err := fs.ReadFile(fsys, "response.binarypb")
		if err != nil {
			return resp, err
		}

		if err := proto.Unmarshal(responseBytes, resp); err != nil {
			return resp, err
		}

		attachToAction(ctx, "response", resp, oo.RedactResponse)
		return resp, nil
	})
}

func redact(m proto.Message, f func(proto.Message) proto.Message) proto.Message {
	if f == nil {
		return m
	}
	return f(m)
}

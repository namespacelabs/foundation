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

type LowLevelInvokeOptions struct {
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

func InvokeOnBuildkit[Resp proto.Message](c *buildkit.GatewayClient, method string, pkg schema.PackageName, image compute.Computable[oci.Image], opts rtypes.RunBinaryOpts, req proto.Message, oo LowLevelInvokeOptions) compute.Computable[Resp] {
	state := makeState(c, pkg, image, method, req, opts, oo)

	p := c.BuildkitOpts().HostPlatform

	files := buildkit.DeferBuildFilesystem(c, build.NewBuildTarget(&p).WithSourceLabel("Invocation %s", pkg).WithSourcePackage(pkg), state)

	return compute.Transform("parse-response", files, func(ctx context.Context, fsys fs.FS) (Resp, error) {
		resp := protos.NewFromType[Resp]()

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

func makeState(c *buildkit.GatewayClient, pkg schema.PackageName, image compute.Computable[oci.Image], method string, req proto.Message, opts rtypes.RunBinaryOpts, oo LowLevelInvokeOptions) compute.Computable[llb.State] {
	return compute.Transform("make-request", ensureCached(image), func(ctx context.Context, image oci.Image) (llb.State, error) {
		attachToAction(ctx, "request", req, oo.RedactRequest)

		d, err := image.Digest()
		if err != nil {
			return llb.State{}, err
		}

		tasks.Attachments(ctx).AddResult("ref", d.String())

		if !c.BuildkitOpts().SupportsCanonicalBuilds {
			return llb.State{}, fnerrors.InvocationError("buildkit", "the target buildkit does not have the required capabilities (ocilayout input), please upgrade")
		}

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
			return llb.State{}, err
		}

		requestState := llb.Scratch().File(llb.Mkfile("request.binarypb", 0644, requestBytes))

		run.AddMount("/request", requestState, llb.Readonly)
		out := run.AddMount("/out", llb.Scratch())
		return out, nil
	})
}

func ensureCached(image compute.Computable[oci.Image]) compute.Computable[oci.Image] {
	return compute.Transform("ensure-cached", image, oci.EnsureCached)
}

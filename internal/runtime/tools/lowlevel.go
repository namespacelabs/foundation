// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package tools

import (
	"context"
	"fmt"
	"io/fs"
	"net"
	"os"
	"reflect"
	"time"

	"github.com/moby/buildkit/client/llb"
	"github.com/opencontainers/go-digest"
	"golang.org/x/exp/slices"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/build"
	"namespacelabs.dev/foundation/internal/build/buildkit"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/environment"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/grpcstdio"
	"namespacelabs.dev/foundation/internal/planning/tool/protocol"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/internal/runtime/rtypes"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/tasks"
)

var LowLevelToolsProtocolVersion = 2
var InvocationCanUseBuildkit = false

const (
	MaxInvocationDuration = 1 * time.Minute
)

type ResolveMethodFunc[Req, Resp proto.Message] func(*grpc.ClientConn) func(context.Context, Req, ...grpc.CallOption) (Resp, error)

type LowLevelInvokeOptions[Req, Resp proto.Message] struct {
	RedactRequest  func(proto.Message) proto.Message
	RedactResponse func(proto.Message) proto.Message
}

func CanUseBuildkit(env cfg.Configuration) bool {
	return InvocationCanUseBuildkit
}

func attachToAction(ctx context.Context, name string, msg proto.Message, redactMessage func(proto.Message) proto.Message) {
	attachments := tasks.Attachments(ctx)
	err := attachments.AttachSerializable(name+".textpb", "", redact(msg, redactMessage))
	if err != nil {
		fmt.Fprintf(console.Debug(ctx), "failed to serialize request: %v", err)
	}
}

func (oo LowLevelInvokeOptions[Req, Resp]) Invoke(ctx context.Context, conf cfg.Configuration, pkg schema.PackageName, opts rtypes.RunToolOpts, req Req, resolve ResolveMethodFunc[Req, Resp]) (Resp, error) {
	// XXX security: think through whether it is OK or not to expose Snapshots here.
	// For now, assume not.
	attachToAction(ctx, "request", req, oo.RedactRequest)

	// The request and response are attached to the parent context.

	var resp Resp
	if err := tasks.Action("lowlevel.invocation").Arg("package", pkg).LogLevel(1).Run(ctx, func(ctx context.Context) error {
		// os.Pipe is used instead of io.Pipe, as exec.Command will anyway behind the scenes
		// create an additional io.Pipe to copy back to os.Pipe; as we need real pipes to communicate
		// with the underlying process.
		outr, outw, err := os.Pipe()
		if err != nil {
			return err
		}

		defer outr.Close()
		defer outw.Close()

		inr, inw, err := os.Pipe()
		if err != nil {
			return err
		}

		defer inr.Close()
		defer inw.Close()

		opts.Stdin = inr
		opts.Stdout = outw
		opts.Stderr = console.Output(ctx, pkg.String())

		if !environment.IsRunningInCI() {
			ctxWithTimeout, cancel := context.WithTimeout(ctx, MaxInvocationDuration)
			defer cancel()
			ctx = ctxWithTimeout
		}

		eg := executor.New(ctx, fmt.Sprintf("lowlevel.invoke(%s)", pkg))

		eg.Go(func(ctx context.Context) error {
			return RunWithOpts(ctx, conf, opts, func() {
				version := LowLevelToolsProtocolVersion
				if opts.SupportedToolVersion != 0 {
					version = opts.SupportedToolVersion
				}

				// Only kick off the session after the binary is started; i.e. after the underlying
				// image has been loaded. In CI in particular, access to docker has high contention and
				// we see up to 20 secs waiting time loading an image.
				eg.Go(func(ctx context.Context) error {
					session, err := grpcstdio.NewSession(ctx, outr, inw, grpcstdio.WithVersion(version), grpcstdio.WithDefaults())
					if err != nil {
						return err
					}

					conn, err := grpc.DialContext(ctx, "stdio",
						grpc.WithTransportCredentials(insecure.NewCredentials()),
						grpc.WithReadBufferSize(0),
						grpc.WithWriteBufferSize(0),
						grpc.WithContextDialer(func(_ context.Context, _ string) (net.Conn, error) {
							return session.Dial(&grpcstdio.DialArgs{
								StreamType:  grpcstdio.DialArgs_STREAM_TYPE_GRPC,
								ServiceName: protocol.InvocationService_ServiceDesc.ServiceName,
							})
						}))
					if err != nil {
						return err
					}

					defer conn.Close()

					resp, err = resolve(conn)(ctx, req) // XXX lock?
					return err
				})
			})
		})

		if err := eg.Wait(); err != nil {
			return err
		}

		if reflect.ValueOf(resp).IsNil() {
			return fnerrors.InternalError("never produced a response")
		}

		return nil
	}); err != nil {
		return resp, err
	}

	attachToAction(ctx, "response", resp, oo.RedactResponse)

	return resp, nil
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

// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tools

import (
	"context"
	"fmt"
	"net"
	"os"
	"reflect"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/environment"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/grpcstdio"
	"namespacelabs.dev/foundation/provision/tool/protocol"
	"namespacelabs.dev/foundation/runtime/rtypes"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/tasks"
)

var LowLevelToolsProtocolVersion = 2

const (
	MaxInvocationDuration = 1 * time.Minute
)

type ResolveMethodFunc[Req, Resp proto.Message] func(*grpc.ClientConn) func(context.Context, Req, ...grpc.CallOption) (Resp, error)

type LowLevelInvokeOptions[Req, Resp proto.Message] struct {
	RedactRequest  func(proto.Message) proto.Message
	RedactResponse func(proto.Message) proto.Message
}

func (oo LowLevelInvokeOptions[Req, Resp]) Invoke(ctx context.Context, pkg schema.PackageName, opts rtypes.RunToolOpts, req Req, resolve ResolveMethodFunc[Req, Resp]) (Resp, error) {
	// XXX security: think through whether it is OK or not to expose Snapshots here.
	// For now, assume not.
	attachments := tasks.Attachments(ctx)
	err := attachments.AttachSerializable("request.textpb", "", redact(req, oo.RedactRequest))
	if err != nil {
		fmt.Fprintf(console.Debug(ctx), "failed to serialize request: %v", err)
	}

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
			return RunWithOpts(ctx, opts, func() {
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

	err = attachments.AttachSerializable("response.textpb", "", redact(resp, oo.RedactResponse))
	if err != nil {
		fmt.Fprintf(console.Debug(ctx), "failed to serialize response: %v", err)
	}

	return resp, nil
}

func redact(m proto.Message, f func(proto.Message) proto.Message) proto.Message {
	if f == nil {
		return m
	}
	return f(m)
}

// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tools

import (
	"context"
	"net"
	"os"
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

const (
	MaxHandshakeTime      = 10 * time.Second // Maximum amount of time we'll wait for a Hello from a worker.
	MaxInvocationDuration = 30 * time.Second
)

func LowLevelInvoke(ctx context.Context, pkg schema.PackageName, opts rtypes.RunToolOpts, req *protocol.ToolRequest) (*protocol.ToolResponse, error) {
	// XXX security: think through whether it is OK or not to expose Snapshots here.
	// For now, assume not.
	attachments := tasks.Attachments(ctx)
	if attachments.IsRecording() {
		reqcopy := proto.Clone(req).(*protocol.ToolRequest)
		reqcopy.Snapshot = nil
		attachments.AttachSerializable("request.textpb", "", reqcopy)
	}

	// os.Pipe is used instead of io.Pipe, as exec.Command will anyway behind the scenes
	// create an additional io.Pipe to copy back to os.Pipe; as we need real pipes to communicate
	// with the underlying process.
	outr, outw, err := os.Pipe()
	if err != nil {
		return nil, err
	}

	defer outr.Close()
	defer outw.Close()

	inr, inw, err := os.Pipe()
	if err != nil {
		return nil, err
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

	eg, wait := executor.New(ctx)

	eg.Go(func(ctx context.Context) error {
		return Impl().Run(ctx, opts)
	})

	var resp *protocol.ToolResponse // XXX lock.
	eg.Go(func(ctx context.Context) error {
		session, err := grpcstdio.NewSession(ctx, outr, inw)
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

		resp, err = protocol.NewInvocationServiceClient(conn).Invoke(ctx, req)
		return err
	})

	if err := wait(); err != nil {
		return nil, err
	}

	if resp == nil {
		return nil, fnerrors.InternalError("never produced a response")
	}

	attachments.AttachSerializable("response.textpb", "", resp)

	return resp, nil
}

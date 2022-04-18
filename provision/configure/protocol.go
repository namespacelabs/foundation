// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package configure

import (
	"context"
	"io"
	"net"
	"os"

	"google.golang.org/grpc"
	"namespacelabs.dev/foundation/internal/grpcstdio"
	"namespacelabs.dev/foundation/internal/versions"
	"namespacelabs.dev/foundation/provision/tool/protocol"
)

func handle(ctx context.Context, h AllHandlers) error {
	conn, err := grpc.DialContext(ctx, "stdio",
		grpc.WithInsecure(),
		grpc.WithReadBufferSize(0),
		grpc.WithWriteBufferSize(0),
		grpc.WithContextDialer(func(_ context.Context, _ string) (net.Conn, error) {
			return grpcstdio.NewConnection(os.Stdout, os.Stdin), nil
		}))
	if err != nil {
		return err
	}

	defer conn.Close()

	cli := protocol.NewInvocationServiceClient(conn)
	stream, err := cli.Worker(ctx)
	if err != nil {
		return err
	}

	if err := stream.Send(&protocol.WorkerChunk{ClientHello: &protocol.WorkerChunk_ClientHello{
		FnApiVersion:   versions.APIVersion,
		ToolApiVersion: versions.ToolAPIVersion,
	}}); err != nil {
		return err
	}

	for {
		msg, err := stream.Recv()
		if err != nil {
			return err
		}

		if msg.ToolRequest != nil {
			response, err := handleRequest(ctx, msg.ToolRequest, h)
			if err != nil {
				return err
			}

			if err := stream.Send(&protocol.WorkerChunk{ToolResponse: response}); err != nil {
				return err
			}

			if err := stream.CloseSend(); err != nil {
				return err
			}

			// Make sure that the send was received.
			if _, err := stream.Recv(); err != nil {
				if err != io.EOF {
					return err
				}
			}

			return nil
		}
	}
}

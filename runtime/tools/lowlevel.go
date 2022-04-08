// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tools

import (
	"context"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/localexec"
	"namespacelabs.dev/foundation/internal/versions"
	"namespacelabs.dev/foundation/provision/tool/grpcstdio"
	"namespacelabs.dev/foundation/provision/tool/protocol"
	"namespacelabs.dev/foundation/runtime/rtypes"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/tasks"
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

	eg, wait := executor.New(ctx)

	lis := grpcstdio.NewListener(ctx)

	eg.Go(func(ctx context.Context) error {
		defer lis.Close() // Signal the server to exit when the client leaves.
		return Impl().RunWithOpts(ctx, opts, localexec.RunOpts{
			OnStart: func() {
				lis.Ready(grpcstdio.NewConnection(inw, outr))
			},
		})
	})

	var resp *protocol.ToolResponse // XXX lock.
	eg.Go(func(ctx context.Context) error {
		s := grpc.NewServer()
		protocol.RegisterInvocationServiceServer(s, service{
			request: req,
			onResponse: func(tr *protocol.ToolResponse) {
				resp = tr
			},
		})
		if err := s.Serve(lis); err != nil {
			// Expected exit.
			if err != grpcstdio.ErrListenerClosed {
				return err
			}
		}
		return nil
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

type service struct {
	request    *protocol.ToolRequest
	onResponse func(*protocol.ToolResponse)
}

func (svc service) Worker(server protocol.InvocationService_WorkerServer) error {
	for {
		chunk, err := server.Recv()
		if err != nil {
			return err
		}

		if chunk.ClientHello != nil {
			if err := server.Send(&protocol.WorkerCoordinatorChunk{
				ServerHello: &protocol.WorkerCoordinatorChunk_ServerHello{
					FnApiVersion: versions.APIVersion,
				},
				ToolRequest: svc.request,
			}); err != nil {
				return err
			}
		}

		if chunk.ToolResponse != nil {
			svc.onResponse(chunk.ToolResponse)
			return nil
		}
	}
}

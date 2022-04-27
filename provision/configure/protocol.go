// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package configure

import (
	"context"
	"os"

	"google.golang.org/grpc"
	"namespacelabs.dev/foundation/internal/grpcstdio"
	"namespacelabs.dev/foundation/provision/tool/protocol"
)

func handle(ctx context.Context, h AllHandlers) error {
	s := grpc.NewServer()

	x, err := grpcstdio.NewSession(ctx, os.Stdin, os.Stdout, grpcstdio.WithCloseNotifier(func(_ *grpcstdio.Stream) {
		// After we're done replying, shutdown the server, and then the binary.
		// But we can't stop the server from this callback, as we're called with
		// grpcstdio locks held, and terminating the server will need to call
		// Close on open connections, which would lead to a deadlock.
		go s.Stop()
	}))
	if err != nil {
		return err
	}

	protocol.RegisterInvocationServiceServer(s, impl{h: h})

	return s.Serve(x.Listener())
}

type impl struct {
	protocol.UnimplementedInvocationServiceServer
	h AllHandlers
}

func (i impl) Invoke(ctx context.Context, req *protocol.ToolRequest) (*protocol.ToolResponse, error) {
	return handleRequest(ctx, req, i.h)
}

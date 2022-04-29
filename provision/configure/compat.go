// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package configure

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"namespacelabs.dev/foundation/provision/tool/protocol"
)

func RunTool(t Tool) {
	run(context.Background(), HandlerCompat{t})
}

type HandlerCompat struct {
	Tool Tool
}

var _ AllHandlers = HandlerCompat{}

func (h HandlerCompat) Apply(ctx context.Context, req StackRequest, output *ApplyOutput) error {
	return h.Tool.Apply(ctx, req, output)
}

func (h HandlerCompat) Delete(ctx context.Context, req StackRequest, output *DeleteOutput) error {
	return h.Tool.Delete(ctx, req, output)
}

func (h HandlerCompat) Invoke(context.Context, Request) (*protocol.InvokeResponse, error) {
	return nil, status.Error(codes.Unavailable, "invoke not supported")
}

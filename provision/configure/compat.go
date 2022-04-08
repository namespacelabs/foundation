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
	run(context.Background(), handlerCompat{t})
}

type handlerCompat struct {
	tool Tool
}

var _ AllHandlers = handlerCompat{}

func (h handlerCompat) Apply(ctx context.Context, req StackRequest, output *ApplyOutput) error {
	return h.tool.Apply(ctx, req, output)
}

func (h handlerCompat) Delete(ctx context.Context, req StackRequest, output *DeleteOutput) error {
	return h.tool.Delete(ctx, req, output)
}

func (h handlerCompat) Invoke(context.Context, Request) (*protocol.InvokeResponse, error) {
	return nil, status.Error(codes.Unavailable, "invoke not supported")
}

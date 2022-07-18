// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package logging

import (
	"context"

	"google.golang.org/grpc/status"
	"namespacelabs.dev/foundation/std/protocol"
)

type contextKey string

var ck contextKey = "ns.ctx.request-id"

type requestData struct {
	rid string
}

func RequestID(ctx context.Context) string {
	v := ctx.Value(ck)
	if v != nil {
		return v.(requestData).rid
	}

	return ""
}

func withRequestID(ctx context.Context, rd requestData) context.Context {
	return context.WithValue(ctx, ck, rd)
}

func AttachRequestIDToError(err error, reqid string) error {
	if err == nil {
		return nil
	}

	st, _ := status.FromError(err)
	tSt, tErr := st.WithDetails(&protocol.RequestID{Id: reqid})
	if err == nil {
		return tSt.Err()
	}

	Log.Printf("[warning] failed to attach %q to error: %v", reqid, tErr)
	return err
}

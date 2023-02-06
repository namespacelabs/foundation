// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package requestid

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
	"namespacelabs.dev/foundation/std/go/core"
	"namespacelabs.dev/foundation/std/protocol"
	"namespacelabs.dev/go-ids"
)

type RequestID string

type contextKey string

var ck contextKey = "ns.ctx.request-id"

type RequestData struct {
	Started   time.Time
	RequestID RequestID
}

func NewID() RequestID {
	return RequestID(ids.NewRandomBase32ID(16))
}

func RequestIDFromContext(ctx context.Context) (RequestID, bool) {
	if data, has := RequestDataFromContext(ctx); has {
		return data.RequestID, true
	}

	return "", false
}

func RequestDataFromContext(ctx context.Context) (RequestData, bool) {
	v := ctx.Value(ck)
	if v != nil {
		return v.(RequestData), true
	}

	return RequestData{}, false
}

func allocateRequestID(ctx context.Context) (context.Context, RequestData) {
	rdata := RequestData{
		Started:   time.Now(),
		RequestID: NewID(),
	}
	return context.WithValue(ctx, ck, rdata), rdata
}

func AttachRequestIDToError(err error, reqid RequestID) error {
	if err == nil {
		return nil
	}

	st, _ := status.FromError(err)
	tSt, tErr := st.WithDetails(&protocol.RequestID{Id: string(reqid)})
	if tErr == nil {
		return tSt.Err()
	}

	core.Log.Printf("[warning] failed to attach %q to error: %v", reqid, tErr)
	return err
}

type Interceptor struct{}

func (Interceptor) Unary(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	ctx, rdata := allocateRequestID(ctx)
	resp, unaryErr := handler(ctx, req)
	return resp, AttachRequestIDToError(unaryErr, rdata.RequestID)
}

func (Interceptor) Streaming(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	ctx, rdata := allocateRequestID(stream.Context())
	streamErr := handler(srv, serverStream{stream, ctx})
	return AttachRequestIDToError(streamErr, rdata.RequestID)
}

type serverStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s serverStream) Context() context.Context { return s.ctx }

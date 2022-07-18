// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
	"namespacelabs.dev/foundation/std/grpc/requestid"
)

const maxOutputToTerminal = 128

var Log = log.New(os.Stderr, "[grpclog] ", log.Ldate|log.Ltime|log.Lmicroseconds)

type interceptor struct{}

func (interceptor) unary(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	rdata, has := requestid.RequestDataFromContext(ctx)
	if !has {
		return handler(ctx, req)
	}

	logHeader(ctx, rdata.RequestID, "request", info.FullMethod, req)

	resp, err := handler(ctx, req)
	if err == nil {
		Log.Printf("%s: id=%s: took %v; response: %s", info.FullMethod, rdata.RequestID, time.Since(rdata.Started), serializeMessage(resp))
	} else {
		Log.Printf("%s: id=%s: took %v; error: %v", info.FullMethod, rdata.RequestID, time.Since(rdata.Started), err)
	}
	return resp, err
}

func (interceptor) streaming(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	rdata, has := requestid.RequestDataFromContext(stream.Context())
	if !has {
		return handler(srv, stream)
	}

	logHeader(stream.Context(), rdata.RequestID, "stream", info.FullMethod, nil)
	err := handler(srv, stream)
	if err == nil {
		Log.Printf("%s: id=%s: took %v, finished ok", info.FullMethod, rdata.RequestID, time.Since(rdata.Started))
	} else {
		Log.Printf("%s: id=%s: took %v; error: %v", info.FullMethod, rdata.RequestID, time.Since(rdata.Started), err)
	}
	return err
}

func logHeader(ctx context.Context, reqid, what, fullMethod string, req interface{}) {
	peerAddr := "unknown"
	authType := "none"
	deadline := "none"
	if p, has := peer.FromContext(ctx); has {
		peerAddr = p.Addr.String()
		if p.AuthInfo != nil {
			authType = p.AuthInfo.AuthType()
		}
	}

	if t, ok := ctx.Deadline(); ok {
		left := time.Until(t)
		deadline = fmt.Sprintf("%v", left)
	}

	if req != nil {
		Log.Printf("%s: id=%s: request from %s (auth: %s, deadline: %s): %s", fullMethod, reqid, peerAddr, authType, deadline, serializeMessage(req))
	} else {
		Log.Printf("%s: id=%s: request from %s (auth: %s, deadline: %s)", fullMethod, reqid, peerAddr, authType, deadline)
	}
}

func Prepare(ctx context.Context, deps ExtensionDeps) error {
	var interceptor interceptor
	deps.Interceptors.ForServer(interceptor.unary, interceptor.streaming)
	return nil
}

func serializeMessage(msg interface{}) string {
	if msg == nil {
		return "<nil>"
	}

	reqBytes, _ := json.Marshal(msg)
	reqStr := string(reqBytes)
	if len(reqStr) > maxOutputToTerminal {
		return fmt.Sprintf("%s [...%d chars truncated]", reqStr[:maxOutputToTerminal], len(reqStr)-maxOutputToTerminal)
	}
	return reqStr
}

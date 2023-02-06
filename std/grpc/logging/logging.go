// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"namespacelabs.dev/foundation/std/grpc/requestid"
)

const maxOutputToTerminal = 1024

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

func ParsePeerAddress(p *peer.Peer, md metadata.MD) (string, string) {
	peerAddr := "unknown"
	originalAddr := ""

	if realIp := single(md, "x-real-ip"); realIp != "" {
		peerAddr = fmt.Sprintf("%s (saw %s)", realIp, peerAddr)
		originalAddr = peerAddr
		peerAddr = realIp

		// XXX use conditional printing instead.
		delete(md, "x-real-ip")
		delete(md, "x-forwarded-for")
		delete(md, "x-forwarded-host")
		delete(md, "x-forwarded-port")
		delete(md, "x-forwarded-proto")
		delete(md, "x-forwarded-scheme")
		delete(md, "x-scheme")
	}

	return peerAddr, originalAddr
}

func logHeader(ctx context.Context, reqid requestid.RequestID, what, fullMethod string, req interface{}) {
	authType := "none"
	deadline := "none"

	p, _ := peer.FromContext(ctx)
	if p != nil && p.AuthInfo != nil {
		authType = p.AuthInfo.AuthType()
	}

	// It's OK to modify the map below, because `FromIncomingContext` returns a copy.
	md, _ := metadata.FromIncomingContext(ctx)

	delete(md, "accept-encoding")
	delete(md, "content-type")

	authority := single(md, ":authority")
	delete(md, ":authority")

	if t, ok := ctx.Deadline(); ok {
		left := time.Until(t)
		deadline = fmt.Sprintf("%v", left)
	}

	peerAddr, wasAddr := ParsePeerAddress(p, md)
	if wasAddr != "" {
		peerAddr += fmt.Sprintf(" (saw %s)", wasAddr)
	}

	if _, ok := md["authorization"]; ok {
		authType = fmt.Sprintf("bearer (was %s)", authType)
		delete(md, "authorization")
	}

	if req != nil {
		Log.Printf("%s: id=%s: request from %s to %s (auth: %s, deadline: %s, metadata: %+v): %s", fullMethod, reqid, peerAddr, authority, authType, deadline, md, serializeMessage(req))
	} else {
		Log.Printf("%s: id=%s: request from %s to %s (auth: %s, deadline: %s, metadata: %+v)", fullMethod, reqid, peerAddr, authority, authType, deadline, md)
	}
}

func single(md metadata.MD, key string) string {
	if value, ok := md[key]; ok && len(value) == 1 {
		return value[0]
	}
	return ""
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

// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	nsgrpc "namespacelabs.dev/foundation/std/grpc"
	"namespacelabs.dev/foundation/std/grpc/requestid"
)

const maxOutputToTerminal = 1024

func init() {
	zerolog.TimeFieldFormat = time.RFC3339Nano // Setting external package globals does not make me happy.
}

var Log = zerolog.New(os.Stderr).With().Timestamp().Logger().Level(zerolog.DebugLevel)

func Background() context.Context {
	return Log.WithContext(context.Background())
}

type interceptor struct{}

func (interceptor) unary(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	rdata, has := requestid.RequestDataFromContext(ctx)
	if !has {
		return handler(ctx, req)
	}

	zero := prepareLogger(ctx, rdata.RequestID, info.FullMethod)
	logger := zero.Logger()

	attachRequestData(ctx, logger.Info().Str("kind", "grpclog").Str("what", "request")).Str("request_body", serializeMessage(req)).Send()

	resp, err := handler(logger.WithContext(ctx), req)
	if err == nil {
		logger.Info().Str("kind", "grpclog").Dur("took", time.Since(rdata.Started)).Str("what", "response").Str("response_body", serializeMessage(resp)).Send()
	} else {
		logger.Info().Str("kind", "grpclog").Dur("took", time.Since(rdata.Started)).Str("what", "response").Err(err).Send()
	}
	return resp, err
}

func (interceptor) streaming(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	ctx := stream.Context()

	rdata, has := requestid.RequestDataFromContext(ctx)
	if !has {
		return handler(srv, stream)
	}

	zero := prepareLogger(ctx, rdata.RequestID, info.FullMethod)
	logger := zero.Logger()

	attachRequestData(ctx, logger.Info().Str("kind", "grpclog").Str("what", "stream_start")).Send()

	err := handler(srv, &serverStream{stream, logger.WithContext(ctx)})
	if err == nil {
		logger.Info().Str("kind", "grpclog").Dur("took", time.Since(rdata.Started)).Str("what", "stream_end").Send()
	} else {
		logger.Info().Str("kind", "grpclog").Dur("took", time.Since(rdata.Started)).Str("what", "stream_end").Err(err).Send()
	}
	return err
}

func ParsePeerAddress(p *peer.Peer, md metadata.MD) (string, string) {
	peerAddr := "unknown"
	originalAddr := ""

	if p != nil {
		peerAddr = p.Addr.String()
	}

	if realIp := single(md, "x-real-ip"); realIp != "" {
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

func prepareLogger(ctx context.Context, reqid requestid.RequestID, fullMethod string) zerolog.Context {
	service, method := nsgrpc.SplitMethodName(fullMethod)

	return Log.With().Str("service", service).Str("method", method).
		Str("request_id", string(reqid))
}

func attachRequestData(ctx context.Context, ev *zerolog.Event) *zerolog.Event {
	var authType string
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

	if _, ok := md["authorization"]; ok {
		if authType != "" {
			authType = fmt.Sprintf("bearer (was %s)", authType)
		} else {
			authType = "bearer"
		}
		delete(md, "authorization")
	}

	peerAddr, wasAddr := ParsePeerAddress(p, md)
	ev = ev.Str("peer", peerAddr)
	if wasAddr != "" {
		ev = ev.Str("original_peer", wasAddr)
	}

	if authType != "" {
		ev = ev.Str("auth_type", authType)
	}

	if authority != "" {
		ev = ev.Str("authority", authority)
	}

	if t, ok := ctx.Deadline(); ok {
		ev = ev.Dur("deadline_left", time.Until(t))
	}

	return ev
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
	deps.Middleware.Add(func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rdata, has := requestid.RequestDataFromContext(r.Context())

			log := Log.With().Str("http_method", r.Method).Stringer("http_url", r.URL)
			if has {
				log = log.Str("request_id", string(rdata.RequestID))
			}

			logger := log.Logger()

			h.ServeHTTP(w, r.WithContext(logger.WithContext(r.Context())))
		})
	})
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

type serverStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *serverStream) Context() context.Context {
	return w.ctx
}

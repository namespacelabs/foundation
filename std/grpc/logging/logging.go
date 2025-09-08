// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"golang.org/x/exp/slices"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"namespacelabs.dev/foundation/std/go/core"
	nsgrpc "namespacelabs.dev/foundation/std/grpc"
	"namespacelabs.dev/foundation/std/grpc/requestid"
)

func init() {
	zerolog.TimeFieldFormat = time.RFC3339Nano // Setting external package globals does not make me happy.
}

func fromEnv() (int, []string) {
	var maxOutputToTerminal = 1024
	var skipLogging []string

	if v := os.Getenv("FOUNDATION_GRPCLOG_MESSAGE_MAX_BYTES"); v != "" {
		if parsed, err := strconv.ParseInt(v, 10, 32); err != nil {
			panic(err)
		} else {
			maxOutputToTerminal = int(parsed)
		}
	}
	if v := os.Getenv("FOUNDATION_GRPCLOG_SKIP_METHODS"); v != "" {
		skipLogging = strings.Split(v, ",")
	}

	return maxOutputToTerminal, skipLogging
}

var Log = core.ZLog

func Background() context.Context {
	return Log.WithContext(context.Background())
}

type Interceptor struct {
	Logger *zerolog.Logger

	MaxOutputToTerminal int
	SkipLogging         []string
}

func (ic Interceptor) Unary(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	rdata, has := requestid.RequestDataFromContext(ctx)
	if !has {
		return handler(ctx, req)
	}

	logger := ic.prepareLogger(rdata.RequestID, info.FullMethod)
	loggable := !slices.Contains(ic.SkipLogging, strings.TrimPrefix(info.FullMethod, "/"))

	if loggable {
		makeNewEvent(ctx, logger.Info().Str("kind", "grpclog").Str("what", "request")).Str("request_body", serializeMessage(req, ic.MaxOutputToTerminal)).Send()
	}

	resp, err := handler(logger.WithContext(ctx), req)
	if loggable {
		if err == nil {
			logger.Info().Str("kind", "grpclog").Dur("took", time.Since(rdata.Started)).Str("what", "response").Str("response_body", serializeMessage(resp, ic.MaxOutputToTerminal)).Send()
		} else {
			st, ok := status.FromError(err)

			var detailTypes []string
			for _, det := range st.Proto().GetDetails() {
				detailTypes = append(detailTypes, det.TypeUrl)
			}

			logger.Err(err).Str("kind", "grpclog").Dur("took", time.Since(rdata.Started)).Str("what", "response").
				Bool("status_ok", ok).Strs("error_detail_types", detailTypes).Send()
		}
	}
	return resp, err
}

func (ic Interceptor) Streaming(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	ctx := stream.Context()

	rdata, has := requestid.RequestDataFromContext(ctx)
	if !has {
		return handler(srv, stream)
	}

	logger := ic.prepareLogger(rdata.RequestID, info.FullMethod)
	loggable := !slices.Contains(ic.SkipLogging, strings.TrimPrefix(info.FullMethod, "/"))

	if loggable {
		makeNewEvent(ctx, logger.Info().Str("kind", "grpclog").Str("what", "stream_start")).Send()
	}

	err := handler(srv, &serverStream{stream, logger.WithContext(ctx)})
	if loggable {
		if err == nil {
			logger.Info().Str("kind", "grpclog").Dur("took", time.Since(rdata.Started)).Str("what", "stream_end").Send()
		} else {
			logger.Info().Str("kind", "grpclog").Dur("took", time.Since(rdata.Started)).Str("what", "stream_end").Err(err).Send()
		}
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

func (ic Interceptor) prepareLogger(reqid requestid.RequestID, fullMethod string) zerolog.Logger {
	zl := Log
	if ic.Logger != nil {
		zl = *ic.Logger
	}

	service, method := nsgrpc.SplitMethodName(fullMethod)

	return zl.With().Str("service", service).Str("method", method).
		Str("request_id", string(reqid)).Logger()
}

func makeNewEvent(ctx context.Context, ev *zerolog.Event) *zerolog.Event {
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
	n, s := fromEnv()

	interceptor := Interceptor{
		MaxOutputToTerminal: n,
		SkipLogging:         s,
	}

	deps.Interceptors.ForServer(interceptor.Unary, interceptor.Streaming)
	return nil
}

func serializeMessage(msg interface{}, max int) string {
	if msg == nil {
		return "<nil>"
	}

	reqBytes, _ := json.Marshal(msg)
	reqStr := string(reqBytes)
	if len(reqStr) > max {
		return fmt.Sprintf("%s [...%d chars truncated]", reqStr[:max], len(reqStr)-max)
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

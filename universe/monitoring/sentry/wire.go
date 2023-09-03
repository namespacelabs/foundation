// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package sentry

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/getsentry/sentry-go"
	sentryhttp "github.com/getsentry/sentry-go/http"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	nsgrpc "namespacelabs.dev/foundation/std/grpc"
	"namespacelabs.dev/foundation/std/grpc/logging"
	"namespacelabs.dev/foundation/std/grpc/requestid"
)

func Prepare(ctx context.Context, deps ExtensionDeps) error {
	key := os.Getenv("MONITORING_SENTRY_DSN")
	if key == "" {
		return nil
	}

	if err := sentry.Init(sentry.ClientOptions{
		Dsn:              key,
		ServerName:       deps.ServerInfo.ServerName,
		Environment:      deps.ServerInfo.EnvName,
		Release:          deps.ServerInfo.GetVcs().GetRevision(),
		TracesSampleRate: 1.0, // XXX should be configurable.
		AttachStacktrace: true,
	}); err != nil {
		return err
	}

	deps.Interceptors.ForServer(unaryInterceptor, streamInterceptor)
	deps.Middleware.Add(sentryhttp.New(sentryhttp.Options{Repanic: true}).Handle)

	return nil
}

func unaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (result interface{}, err error) {
	hub := sentry.GetHubFromContext(ctx)
	if hub == nil {
		hub = sentry.CurrentHub().Clone()
		ctx = sentry.SetHubOnContext(ctx, hub)
	}

	defer recoverAndReport(hub)

	configureScope(ctx, hub, info.FullMethod)
	result, err = handler(ctx, req)
	maybeAttachError(hub, err)
	return result, err
}

func streamInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	ctx := ss.Context()
	hub := sentry.GetHubFromContext(ctx)
	if hub == nil {
		hub = sentry.CurrentHub().Clone()
		ctx = sentry.SetHubOnContext(ctx, hub)
	}

	defer recoverAndReport(hub)

	configureScope(ctx, hub, info.FullMethod)
	err := handler(srv, &serverStream{ServerStream: ss, ctx: ctx})
	maybeAttachError(hub, err)
	return err
}

func configureScope(ctx context.Context, hub *sentry.Hub, fullMethod string) {
	scope := hub.Scope()

	service, method := nsgrpc.SplitMethodName(fullMethod)
	scope.SetTags(map[string]string{
		"grpc.service": service,
		"grpc.method":  method,
	})

	rdata, has := requestid.RequestDataFromContext(ctx)
	if has {
		scope.SetContext("request_id", map[string]any{
			"request_id": rdata.RequestID,
		})
	}

	p, _ := peer.FromContext(ctx)
	md, _ := metadata.FromIncomingContext(ctx)
	peerAddr, originalAddr := logging.ParsePeerAddress(p, md)

	grpcData := map[string]any{
		"peer": peerAddr,
	}

	if originalAddr != "" {
		grpcData["original_peer"] = originalAddr
	}

	scope.SetContext("grpc", grpcData)
}

func maybeAttachError(hub *sentry.Hub, err error) {
	if err != nil {
		grpcCode := errorStatus(err)
		if hub != nil && !isUserError(grpcCode) {
			hub.CaptureException(err)
		}
	}
}

func MaybeAttachError(ctx context.Context, err error) {
	maybeAttachError(sentry.GetHubFromContext(ctx), err)
}

func isUserError(code codes.Code) bool {
	switch code {
	case codes.Canceled, codes.InvalidArgument, codes.NotFound, codes.PermissionDenied, codes.Unauthenticated:
		return true
	}

	return false
}

func errorStatus(err error) codes.Code {
	if errors.Is(err, context.Canceled) {
		return codes.Canceled
	}

	if st, ok := status.FromError(err); ok {
		return st.Code()
	}

	return codes.Unknown
}

func statusFromGrpc(code codes.Code) sentry.SpanStatus {
	switch code {
	case codes.OK:
		return sentry.SpanStatusOK
	case codes.InvalidArgument:
		return sentry.SpanStatusInvalidArgument
	case codes.DeadlineExceeded:
		return sentry.SpanStatusDeadlineExceeded
	case codes.NotFound:
		return sentry.SpanStatusNotFound
	case codes.AlreadyExists:
		return sentry.SpanStatusAlreadyExists
	case codes.PermissionDenied:
		return sentry.SpanStatusPermissionDenied
	case codes.ResourceExhausted:
		return sentry.SpanStatusResourceExhausted
	case codes.FailedPrecondition:
		return sentry.SpanStatusFailedPrecondition
	case codes.Aborted:
		return sentry.SpanStatusAborted
	case codes.OutOfRange:
		return sentry.SpanStatusOutOfRange
	case codes.Unimplemented:
		return sentry.SpanStatusUnimplemented
	case codes.Internal:
		return sentry.SpanStatusInternalError
	case codes.Unavailable:
		return sentry.SpanStatusUnavailable
	case codes.DataLoss:
		return sentry.SpanStatusDataLoss
	case codes.Unauthenticated:
		return sentry.SpanStatusUnauthenticated
	}

	return sentry.SpanStatusUnknown
}

func recoverAndReport(hub *sentry.Hub) {
	if err := recover(); err != nil {
		eventID := hub.Recover(err)
		if eventID != nil {
			hub.Flush(2 * time.Second)
		}
		panic(err)
	}
}

type serverStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *serverStream) Context() context.Context {
	return w.ctx
}

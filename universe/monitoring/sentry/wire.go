// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package sentry

import (
	"context"
	"time"

	"github.com/getsentry/sentry-go"
	sentryhttp "github.com/getsentry/sentry-go/http"
	"google.golang.org/grpc"
)

func Prepare(ctx context.Context, deps ExtensionDeps) error {
	if err := sentry.Init(sentry.ClientOptions{
		Dsn:              string(deps.Dsn.MustValue()),
		ServerName:       deps.ServerInfo.ServerName,
		Environment:      deps.ServerInfo.EnvName,
		Release:          deps.ServerInfo.GetVcs().GetRevision(),
		TracesSampleRate: 1.0, // XXX should be configurable.
	}); err != nil {
		return err
	}

	deps.Interceptors.Add(unaryInterceptor, streamInterceptor)
	deps.Middleware.Add(sentryhttp.New(sentryhttp.Options{}).Handle)

	return nil
}

func unaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (result interface{}, err error) {
	hub := sentry.GetHubFromContext(ctx)
	if hub == nil {
		hub = sentry.CurrentHub().Clone()
		ctx = sentry.SetHubOnContext(ctx, hub)
	}

	span := sentry.StartSpan(ctx, "grpc.server", sentry.TransactionName(info.FullMethod))
	defer span.Finish()

	defer recoverAndReport(ctx, hub)

	result, err = handler(ctx, req)
	if err != nil {
		hub.CaptureException(err)
	}

	return result, err
}

func streamInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	ctx := ss.Context()
	hub := sentry.GetHubFromContext(ctx)
	if hub == nil {
		hub = sentry.CurrentHub().Clone()
		ctx = sentry.SetHubOnContext(ctx, hub)
	}

	span := sentry.StartSpan(ctx, "grpc.server", sentry.TransactionName(info.FullMethod))
	defer span.Finish()

	defer recoverAndReport(ctx, hub)

	err := handler(srv, &serverStream{ServerStream: ss, ctx: ctx})
	if err != nil {
		hub.CaptureException(err)
	}

	return err
}

func recoverAndReport(ctx context.Context, hub *sentry.Hub) {
	if err := recover(); err != nil {
		eventID := hub.RecoverWithContext(ctx, err)
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

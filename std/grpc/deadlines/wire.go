// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package deadlines

import (
	"context"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
)

var (
	mu            sync.RWMutex
	registrations []*DeadlineRegistration
)

type interceptor struct{}

func (interceptor) unary(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	service, method := splitMethodName(info.FullMethod)

	var selected *Deadline_Configuration
	if service != "" && method != "" {
		mu.RLock()
	outer:
		for _, reg := range registrations {
			for _, conf := range reg.conf.GetConfiguration() {
				if (conf.ServiceName == "*" || conf.ServiceName == service) && (conf.MethodName == "*" || conf.MethodName == method) {
					selected = conf
					break outer
				}
			}
		}
		mu.RUnlock()
	}

	if selected != nil {
		// Go will already make sure that we can't increase the incoming deadline.
		newCtx, cancel := context.WithTimeout(ctx, time.Duration(selected.MaximumDeadline*1000000000))
		defer cancel()

		ctx = newCtx
	}

	return handler(ctx, req)
}

func (interceptor) streaming(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	// XXX streaming deadlines.
	return handler(srv, stream)
}

func Prepare(ctx context.Context, deps ExtensionDeps) error {
	var interceptor interceptor
	deps.Interceptors.Add(interceptor.unary, interceptor.streaming)
	return nil
}

func splitMethodName(fullMethodName string) (string, string) {
	fullMethodName = strings.TrimPrefix(fullMethodName, "/")
	if i := strings.Index(fullMethodName, "/"); i >= 0 {
		return fullMethodName[:i], fullMethodName[i+1:]
	}
	return "", ""
}

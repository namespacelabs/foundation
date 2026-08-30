// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package grpc

import (
	"context"
	"flag"
	"fmt"
	"net"
	"strconv"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"namespacelabs.dev/foundation/std/go/core"
	"namespacelabs.dev/foundation/std/go/grpc/client"
	"namespacelabs.dev/foundation/std/go/server"
)

const grpcConnMapKeyword = "grpc_conn_map"

var connMapStr = flag.String(grpcConnMapKeyword, "", "{caller_package}:{owner_package}/{owner_service}={endpoint}")

func ProvideConn(ctx context.Context, req *Backend) (*grpc.ClientConn, error) {
	key := fmt.Sprintf("%s:%s/%s", core.InstantiationPathFromContext(ctx).Last(), req.PackageName, req.ServiceName)

	endpoint := connMapFromArgs()[key]
	if endpoint == "" {
		// If there's no endpoint configured, assume we're doing a loopback.
		return Loopback()
	}

	// XXX ServerResource wrapping is missing.

	return client.NewClient(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials())) ///  XXX mTLS etc.
}

func Loopback(grpcOpts ...grpc.DialOption) (*grpc.ClientConn, error) {
	endpoint := net.JoinHostPort("127.0.0.1", strconv.Itoa(server.ListenPort()))
	return client.NewClient(endpoint, resolveDialOpts(grpcOpts)...)
}

func resolveDialOpts(opts []grpc.DialOption) []grpc.DialOption {
	if len(opts) > 0 {
		return opts
	}

	return []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
}

func connMapFromArgs() map[string]string {
	return parseConn(*connMapStr)
}

func parseConn(src string) map[string]string {
	m := map[string]string{}

	parts := strings.Split(src, ";")
	for _, p := range parts {
		v := strings.TrimSpace(p)
		if v == "" {
			continue
		}

		kvs := strings.SplitN(v, "=", 2)
		if len(kvs) < 2 {
			core.ZLog.Fatal().Msgf("expected key=value format in --%s", grpcConnMapKeyword)
		}

		m[kvs[0]] = kvs[1]
	}

	return m
}

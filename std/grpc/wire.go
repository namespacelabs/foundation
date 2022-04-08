// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package grpc

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"google.golang.org/grpc"
	fninit "namespacelabs.dev/foundation/std/go/core/init"
	"namespacelabs.dev/foundation/std/go/grpc/server"
)

const grpcConnMapKeyword = "grpc_conn_map"

var connMapStr = flag.String(grpcConnMapKeyword, "", "{caller_package}:{owner_package}/{owner_service}={endpoint}")

func ProvideConn(ctx context.Context, caller fninit.Caller, req *Backend) (*grpc.ClientConn, error) {
	key := fmt.Sprintf("%s:%s/%s", caller.String(), req.PackageName, req.ServiceName)

	endpoint := connMapFromArgs()[key]
	if endpoint == "" {
		// If there's no endpoint configured, assume we're doing a loopback.
		endpoint = fmt.Sprintf("127.0.0.1:%d", server.ListenPort())
	}

	// XXX ServerResource wrapping is missing.

	return grpc.DialContext(ctx, endpoint, grpc.WithInsecure()) ///  XXX mTLS etc.
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
			fninit.Log.Fatalf("expected key=value format in --%s", grpcConnMapKeyword)
		}

		m[kvs[0]] = kvs[1]
	}

	return m
}

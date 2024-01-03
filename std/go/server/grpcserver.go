// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package server

import (
	"context"
	"encoding/json"
	"flag"
	"net"
	"time"

	"google.golang.org/grpc/codes"
	"namespacelabs.dev/foundation/framework/rpcerrors"
	"namespacelabs.dev/foundation/framework/runtime"
	"namespacelabs.dev/foundation/std/go/core"
	"namespacelabs.dev/foundation/std/go/grpc/servercore"
)

var (
	listenHostname = flag.String("listen_hostname", "localhost", "Hostname to listen on.")
	port           = flag.Int("grpcserver_port", 0, "Port to listen on.")
	httpPort       = flag.Int("grpcserver_http_port", 0, "Port to listen HTTP on.")
	httpOptions    = flag.String("grpc_http_options", "{}", "Options to pass to the HTTP server.")
)

const drainTimeout = 30 * time.Second

func ListenPort() int     { return *port }
func HTTPListenPort() int { return *httpPort }

func InitializationDone() {
	core.InitializationDone()
}

type Server = servercore.Server
type Registrar = servercore.Registrar
type RunOpts = servercore.RunOpts

func Listen(ctx context.Context, registerServices func(Server)) error {
	return servercore.Listen(ctx, makeListenerOpts(), registerServices)
}

func Run(ctx context.Context, opts RunOpts) {
	servercore.Run(ctx, opts, makeListenerOpts())
}

func makeListenerOpts() servercore.ListenOpts {
	opts := servercore.ListenOpts{
		CreateListener: servercore.MakeTCPListener(*listenHostname, *port),
		CreateNamedListener: func(ctx context.Context, name string) (net.Listener, error) {
			rt, err := runtime.LoadRuntimeConfig()
			if err != nil {
				return nil, err
			}

			for _, port := range rt.Current.Port {
				if port.Name == name {
					return servercore.MakeTCPListener(*listenHostname, int(port.Port))(ctx)
				}
			}

			return nil, rpcerrors.Errorf(codes.InvalidArgument, "no such server port %q", name)
		},
	}

	if *httpPort != 0 {
		opts.CreateHttpListener = func(ctx context.Context) (net.Listener, servercore.HTTPOptions, error) {
			var parsed servercore.HTTPOptions
			if err := json.Unmarshal([]byte(*httpOptions), &parsed); err != nil {
				return nil, servercore.HTTPOptions{}, rpcerrors.Errorf(codes.Internal, "failed to parse http options: %w", err)
			}

			lst, err := servercore.MakeTCPListener(*listenHostname, *httpPort)(ctx)
			return lst, parsed, err
		}
	}

	return opts
}

// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package server

import (
	"context"
	"flag"
	"time"

	"namespacelabs.dev/foundation/std/go/core"
	"namespacelabs.dev/foundation/std/go/grpc/servercore"
)

var (
	listenHostname = flag.String("listen_hostname", "localhost", "Hostname to listen on.")
	port           = flag.Int("grpcserver_port", 0, "Port to listen on.")
	httpPort       = flag.Int("grpcserver_http_port", 0, "Port to listen HTTP on.")
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
	}
	if *httpPort != 0 {
		opts.CreateHttpListener = servercore.MakeTCPListener(*listenHostname, *httpPort)
	}
	return opts
}

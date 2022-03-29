// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

// This file was automatically generated.
package main

import (
	"context"
	"flag"
	"log"

	"namespacelabs.dev/foundation/std/go/core"
	"namespacelabs.dev/foundation/std/go/grpc/server"
)

func main() {
	flag.Parse()

	resources := core.PrepareEnv("namespacelabs.dev/foundation/std/testdata/gogrpcserver")
	defer resources.Close(context.Background())

	ctx := core.WithResources(context.Background(), resources)

	deps, err := PrepareDeps(ctx)
	if err != nil {
		log.Fatal(err)
	}

	server.InitializationDone()

	server.ListenGRPC(ctx, func(srv *server.Grpc) {
		WireServices(ctx, srv, deps)
	})
}
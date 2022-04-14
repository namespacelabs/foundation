// This file was automatically generated.
package main

import (
	"context"
	"flag"
	"log"

	"namespacelabs.dev/foundation/std/go/core"
	"namespacelabs.dev/foundation/std/go/server"
)

func main() {
	flag.Parse()

	resources := core.PrepareEnv("namespacelabs.dev/foundation/std/testdata/server/multidb")
	defer resources.Close(context.Background())

	ctx := core.WithResources(context.Background(), resources)

	deps, err := PrepareDeps(ctx)
	if err != nil {
		log.Fatal(err)
	}

	server.InitializationDone()

	server.Listen(ctx, func(srv server.Server) {
		WireServices(ctx, srv, deps)
	})
}

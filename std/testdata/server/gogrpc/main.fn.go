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

	resources := core.PrepareEnv("namespacelabs.dev/foundation/std/testdata/server/gogrpc")
	defer resources.Close(context.Background())

	ctx := core.WithResources(context.Background(), resources)

	depgraph := core.NewDependencyGraph()
	RegisterInitializers(depgraph)
	if err := depgraph.RunInitializers(ctx); err != nil {
		log.Fatal(err)
	}

	server.InitializationDone()

	server.Listen(ctx, func(srv server.Server) {
		if errs := WireServices(ctx, srv, depgraph); len(errs) > 0 {
			for _, err := range errs {
				log.Println(err)
			}
			log.Fatalf("%d services failed to initialize.", len(errs))
		}
	})
}

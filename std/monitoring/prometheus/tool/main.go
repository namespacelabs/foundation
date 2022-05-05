// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"flag"
	"log"

	"namespacelabs.dev/foundation/provision/configure"
	"namespacelabs.dev/foundation/schema"
)

var (
	mode = flag.String("mode", "", "Do we provision the prometheus server or client?")
)

func main() {
	flag.Parse()

	h := configure.NewHandlers()
	henv := h.MatchEnv(&schema.Environment{Runtime: "kubernetes"})
	switch *mode {
	case "client":
		henv.HandleStack(configureTargets{})
	case "server":
		henv.HandleStack(configureServer{})
	default:
		log.Fatalf("unknown mode: %s", *mode)
	}
	configure.Handle(h)
}

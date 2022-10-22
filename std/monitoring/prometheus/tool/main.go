// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"flag"
	"log"

	"namespacelabs.dev/foundation/framework/provisioning"
	"namespacelabs.dev/foundation/schema"
)

var (
	mode = flag.String("mode", "", "Do we provision the prometheus server or client?")
)

func main() {
	flag.Parse()

	h := provisioning.NewHandlers()
	henv := h.MatchEnv(&schema.Environment{Runtime: "kubernetes"})
	switch *mode {
	case "client":
		henv.HandleStack(configureTargets{})
	case "server":
		henv.HandleStack(configureServer{})
	default:
		log.Fatalf("unknown mode: %s", *mode)
	}
	provisioning.Handle(h)
}

// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"namespacelabs.dev/foundation/provision/configure"
	"namespacelabs.dev/foundation/schema"
)

func main() {
	h := configure.NewHandlers()
	henv := h.MatchEnv(&schema.Environment{Runtime: "kubernetes"})
	henv.HandleStack(configureTargets{})
	henv.HandleStack(configureServer{})
	configure.Handle(h)
}

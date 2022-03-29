// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"namespacelabs.dev/foundation/provision/tool/bootstrap"
	"namespacelabs.dev/foundation/schema"
)

func main() {
	h := bootstrap.NewRegistration()
	henv := h.MatchEnv(&schema.Environment{Runtime: "kubernetes"})
	henv.Handle(configureTargets{})
	henv.Handle(configureServer{})
	bootstrap.RunWith(h)
}
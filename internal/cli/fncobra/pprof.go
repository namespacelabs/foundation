// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package fncobra

import (
	"context"
	"log"
	"net"
	"net/http"

	"github.com/gorilla/mux"
	"namespacelabs.dev/foundation/internal/fnnet"
)

func ListenPProf() {
	if err := listenPProf(); err != nil {
		log.Printf("pprof: failed to listen: %v", err)
	}
}

func listenPProf() error {
	const target = 6060

	h := mux.NewRouter()
	RegisterPprof(h)
	lst, err := fnnet.ListenPort(context.Background(), "127.0.0.1", 0, target)
	if err != nil {
		return err
	}

	localPort := lst.Addr().(*net.TCPAddr).Port
	log.Printf("pprof: listening on http://127.0.0.1:%d/debug/pprof/", localPort)

	return http.Serve(lst, h)
}

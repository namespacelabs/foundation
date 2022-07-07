// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package fncobra

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/pprof"

	"github.com/gorilla/mux"
	"namespacelabs.dev/foundation/internal/fnnet"
)

func ListenPProf(debugSink io.Writer) {
	if err := listenPProf(debugSink); err != nil {
		fmt.Fprintf(debugSink, "pprof: failed to listen: %v\n", err)
	}
}

func listenPProf(debugSink io.Writer) error {
	const target = 6060

	h := mux.NewRouter()
	RegisterPprof(h)
	lst, err := fnnet.ListenPort(context.Background(), "127.0.0.1", 0, target)
	if err != nil {
		return err
	}

	localPort := lst.Addr().(*net.TCPAddr).Port
	fmt.Fprintf(debugSink, "pprof: listening on http://127.0.0.1:%d/debug/pprof/\n", localPort)

	return http.Serve(lst, h)
}

func RegisterPprof(r *mux.Router) {
	r.PathPrefix("/debug/pprof/").HandlerFunc(pprof.Index)
	r.PathPrefix("/debug/pprof/cmdline").HandlerFunc(pprof.Cmdline)
	r.PathPrefix("/debug/pprof/profile").HandlerFunc(pprof.Profile)
	r.PathPrefix("/debug/pprof/symbol").HandlerFunc(pprof.Symbol)
	r.PathPrefix("/debug/pprof/trace").HandlerFunc(pprof.Trace)
	r.PathPrefix("/debug/pprof/goroutine").HandlerFunc(pprof.Index)
}

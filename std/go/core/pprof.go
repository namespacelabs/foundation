// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package core

import (
	"net/http/pprof"

	"github.com/gorilla/mux"
)

func registerPprofEndpoints(mux *mux.Router) string {
	mux.PathPrefix("/debug/pprof/").HandlerFunc(pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	return "/debug/pprof/"
}

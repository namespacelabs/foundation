// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package core

import (
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func RegisterDebugEndpoints(mux *mux.Router) {
	var endpoints []string

	pprofEndpoint := registerPprofEndpoints(mux)
	endpoints = append(endpoints, pprofEndpoint)

	mux.Handle("/metrics", promhttp.Handler())
	mux.Handle("/livez", livezEndpoint())
	mux.Handle("/readyz", readyzEndpoint())

	endpoints = append(endpoints, "/metrics", "/livez", "/readyz")

	debugHandlers.mu.RLock()
	defer debugHandlers.mu.RUnlock()
	for pkg, handler := range debugHandlers.handlers {
		endpoint := "/debug/" + pkg + "/"
		mux.Handle(endpoint, handler)
		endpoints = append(endpoints, endpoint)
	}

	mux.Handle("/", StatusHandler(endpoints))
}

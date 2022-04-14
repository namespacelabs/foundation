// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package core

import (
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func RegisterDebugEndpoints(mux *mux.Router) {
	registerPprofEndpoints(mux)
	mux.Handle("/metrics", promhttp.Handler())
	mux.Handle("/livez", livezEndpoint())
	mux.Handle("/readyz", readyzEndpoint())
	mux.Handle("/", StatusHandler())

	debugHandlers.mu.RLock()
	defer debugHandlers.mu.RUnlock()
	for pkg, handlers := range debugHandlers.handlers {
		mux.Handle("/debug/"+pkg+"/", handlers)
	}
}

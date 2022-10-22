// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

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

// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package reverseproxy

import (
	"context"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

type ProxyConnFunc func(context.Context, string, string) (net.Conn, error)

func DefaultLocalProxy() ProxyConnFunc {
	return (&net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext
}

func Make(proxyTarget string, pcf ProxyConnFunc) http.HandlerFunc {
	revProxy := httputil.NewSingleHostReverseProxy(&url.URL{Scheme: "http", Host: proxyTarget})

	defaultTransport := http.DefaultTransport.(*http.Transport)

	revProxy.Transport = &http.Transport{
		Proxy:                 nil,
		DialContext:           pcf,
		ForceAttemptHTTP2:     defaultTransport.ForceAttemptHTTP2,
		MaxIdleConns:          defaultTransport.MaxIdleConns,
		IdleConnTimeout:       defaultTransport.IdleConnTimeout,
		TLSHandshakeTimeout:   defaultTransport.TLSHandshakeTimeout,
		ExpectContinueTimeout: defaultTransport.ExpectContinueTimeout,
	}

	return revProxy.ServeHTTP
}

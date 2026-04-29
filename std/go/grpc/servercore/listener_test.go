// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package servercore

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"golang.org/x/net/http2"
)

// TestNewHttp2CapableServer_GoawayOnShutdown verifies that
// NewHttp2CapableServer wires up HTTP/2 graceful shutdown for h2c
// connections (via http2.ConfigureServer): an in-flight request
// completes successfully, and a new request on the same pooled
// connection observes the GOAWAY and is forced to dial a fresh
// connection.
func TestNewHttp2CapableServer_GoawayOnShutdown(t *testing.T) {
	started := make(chan struct{})
	finish := make(chan struct{})

	mux := http.NewServeMux()
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-started:
		default:
			close(started)
		}
		<-finish
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	})
	mux.HandleFunc("/quick", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	srv := NewHttp2CapableServer(mux, HTTPOptions{})

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer lis.Close()

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(lis) }()

	// Build an HTTP/2-only client that talks h2c (cleartext) to our
	// server. AllowHTTP=true plus a plain net.Dial in DialTLSContext
	// is the standard recipe for h2c on the client side.
	transport := &http2.Transport{
		AllowHTTP: true,
		DialTLSContext: func(_ context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
			return net.Dial(network, addr)
		},
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport}

	base := "http://" + lis.Addr().String()

	// Prime the connection: a quick request so the http2 client has an
	// open conn to reuse for the slow request.
	if resp, err := client.Get(base + "/quick"); err != nil {
		t.Fatalf("priming request: %v", err)
	} else {
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}

	// Kick off a long-running request. It will block in the handler
	// until we close `finish`.
	var inflightWG sync.WaitGroup
	inflightWG.Add(1)
	var slowResp *http.Response
	var slowErr error
	go func() {
		defer inflightWG.Done()
		slowResp, slowErr = client.Get(base + "/slow")
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("slow handler never started")
	}

	// Trigger graceful shutdown. With http2.ConfigureServer wired in
	// NewHttp2CapableServer, this fires GOAWAY on the active h2c
	// connection.
	shutdownErr := make(chan error, 1)
	go func() { shutdownErr <- srv.Shutdown(context.Background()) }()

	// Give Shutdown a moment to fire OnShutdown handlers (which run
	// in goroutines from inside http.Server.Shutdown) and for the
	// GOAWAY frame to propagate to the client.
	time.Sleep(200 * time.Millisecond)

	// Issue a follow-up request. The client received GOAWAY on the
	// pooled conn, so http2.Transport will refuse to open a new stream
	// on it and try to dial a fresh conn instead. Dial must fail because
	// Shutdown has closed our listener.
	postShutdownReq, _ := http.NewRequest(http.MethodGet, base+"/quick", nil)
	if resp, err := client.Do(postShutdownReq); err == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		t.Errorf("post-shutdown request unexpectedly succeeded with status %d; expected dial failure after GOAWAY", resp.StatusCode)
	}

	// Release the in-flight handler and verify the original request
	// completes successfully — Shutdown should not have RST'd it.
	close(finish)
	inflightWG.Wait()

	if slowErr != nil {
		t.Fatalf("in-flight request failed during shutdown: %v", slowErr)
	}
	if slowResp.StatusCode != http.StatusOK {
		t.Fatalf("expected in-flight request to return 200, got %d", slowResp.StatusCode)
	}
	body, _ := io.ReadAll(slowResp.Body)
	slowResp.Body.Close()
	if string(body) != "ok" {
		t.Fatalf("unexpected body: %q", body)
	}

	select {
	case err := <-shutdownErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Errorf("Shutdown returned: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Shutdown did not return")
	}

	// Drain the Serve goroutine.
	select {
	case err := <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Errorf("Serve returned: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return")
	}
}

// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package servercore

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"maps"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"github.com/soheilhy/cmux"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/http2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
	"namespacelabs.dev/foundation/std/go/core"
	nsgrpc "namespacelabs.dev/foundation/std/go/grpc"
)

func TestMatchDefaultListeners(t *testing.T) {
	for _, tc := range []struct {
		name          string
		hasTLS        bool
		tlsOnly       bool
		separateAdmin bool
		wantTLS       bool
		wantHTTP      bool
		wantGRPC      bool
	}{
		{name: "plaintext", wantHTTP: true, wantGRPC: true},
		{name: "mTLS compatibility", hasTLS: true, wantTLS: true, wantHTTP: true, wantGRPC: true},
		{name: "mTLS only", hasTLS: true, tlsOnly: true, wantTLS: true},
		{name: "separate admin listener", hasTLS: true, separateAdmin: true, wantTLS: true, wantGRPC: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatalf("listen: %v", err)
			}
			defer listener.Close()

			tlsListener, httpListener, grpcListener := matchDefaultListeners(cmux.New(listener), tc.hasTLS, tc.tlsOnly, !tc.separateAdmin)
			if (tlsListener != nil) != tc.wantTLS || (httpListener != nil) != tc.wantHTTP || (grpcListener != nil) != tc.wantGRPC {
				t.Fatalf("unexpected listeners: tls=%t http=%t grpc=%t", tlsListener != nil, httpListener != nil, grpcListener != nil)
			}
		})
	}
}

func TestSecureDefaultListenerRejectsPlaintextHTTP(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	m := cmux.New(listener)
	matchDefaultListeners(m, true, true, false)
	serveDone := make(chan error, 1)
	go func() {
		serveDone <- m.Serve()
	}()

	conn, err := net.DialTimeout("tcp", listener.Addr().String(), time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	if _, err := io.WriteString(conn, "GET /metrics HTTP/1.1\r\nHost: localhost\r\n\r\n"); err != nil {
		t.Fatalf("write request: %v", err)
	}
	if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("set deadline: %v", err)
	}

	buffer := make([]byte, 1)
	if _, err := conn.Read(buffer); err == nil {
		t.Fatal("plaintext connection was not closed")
	} else if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		t.Fatal("plaintext connection remained open")
	}

	if err := listener.Close(); err != nil {
		t.Fatalf("close listener: %v", err)
	}
	select {
	case <-serveDone:
	case <-time.After(time.Second):
		t.Fatal("cmux did not stop")
	}
}

func TestGrpcServerTLSOnly(t *testing.T) {
	t.Setenv(grpcServerTLSOnlyEnv, "true")
	if !grpcServerTLSOnly() {
		t.Fatal("TLS-only mode is disabled")
	}

	t.Setenv(grpcServerTLSOnlyEnv, "false")
	if grpcServerTLSOnly() {
		t.Fatal("TLS-only mode is enabled")
	}
}

func TestRegisterHTTPServicesDoesNotExposeAdministrativeRoutes(t *testing.T) {
	httpMux := NewHTTPMux()
	s := &ServerImpl{httpMux: httpMux}

	registerHTTPServices(s, func(server Server) {
		server.Scope(&core.Package{}).PathPrefix("/").Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("X-Application-Catch-All", "true")
			w.WriteHeader(http.StatusTeapot)
		}))
	})

	for _, path := range []string{"/metrics", "/livez", "/readyz"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		httpMux.ServeHTTP(response, req)
		if response.Header().Get("X-Application-Catch-All") != "true" {
			t.Errorf("%s was not handled by the application catch-all", path)
		}
	}
}

func TestNewHttp2CapableServer_UpgradeObservability(t *testing.T) {
	var logs bytes.Buffer
	originalLogger := core.ZLog
	core.ZLog = zerolog.New(&logs)
	defer func() { core.ZLog = originalLogger }()

	upgradeRequests := func() map[[3]string]float64 {
		samples := map[[3]string]float64{}
		families, err := prometheus.DefaultGatherer.Gather()
		assert.NoError(t, err)
		for _, family := range families {
			if family.GetName() != "ns_gogrpc_h2c_upgrade_requests_total" {
				continue
			}
			for _, metric := range family.GetMetric() {
				labels := map[string]string{}
				for _, label := range metric.GetLabel() {
					labels[label.GetName()] = label.GetValue()
				}
				samples[[3]string{labels["transport"], labels["settings_header"], labels["handling"]}] = metric.GetCounter().GetValue()
			}
		}
		return samples
	}

	for _, tc := range []struct {
		name           string
		upgrade        []string
		connection     []string
		settings       []string
		tls            bool
		status         int
		attempts       float64
		settingsHeader string
		handling       string
	}{
		{name: "ordinary HTTP1", status: 200},
		{name: "unrelated upgrade", upgrade: []string{"websocket"}, connection: []string{"Upgrade"}, status: 200},
		{name: "not an h2c token", upgrade: []string{"h2c-extra"}, status: 200},
		{name: "missing connection token", upgrade: []string{"h2c"}, settings: []string{""}, status: 200, attempts: 1, settingsHeader: "present", handling: "ignored"},
		{name: "missing settings header", upgrade: []string{"h2c"}, connection: []string{"Upgrade, HTTP2-Settings"}, status: 500, attempts: 1, settingsHeader: "missing", handling: "attempted"},
		{name: "invalid settings", upgrade: []string{"h2c"}, connection: []string{"Upgrade, HTTP2-Settings"}, settings: []string{"!"}, status: 500, attempts: 1, settingsHeader: "present", handling: "attempted"},
		{name: "plaintext upgrade", upgrade: []string{"websocket", "H2C"}, connection: []string{"keep-alive", "Upgrade, http2-settings"}, settings: []string{""}, status: 101, attempts: 1, settingsHeader: "present", handling: "attempted"},
		{name: "TLS upgrade", upgrade: []string{"h2c"}, connection: []string{"Upgrade, HTTP2-Settings"}, settings: []string{""}, tls: true, status: 101, attempts: 1, settingsHeader: "present", handling: "attempted"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			logs.Reset()
			server := httptest.NewUnstartedServer(nil)
			server.Config = NewHttp2CapableServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.WriteString(w, "application response")
			}), HTTPOptions{})
			if tc.tls {
				server.StartTLS()
			} else {
				server.Start()
			}
			defer server.Close()

			transport := "plaintext"
			var conn net.Conn
			var err error
			if tc.tls {
				transport = "tls"
				conn, err = tls.Dial("tcp", server.Listener.Addr().String(), server.Client().Transport.(*http.Transport).TLSClientConfig)
			} else {
				conn, err = net.DialTimeout("tcp", server.Listener.Addr().String(), time.Second)
			}
			if !assert.NoError(t, err) {
				return
			}
			defer conn.Close()
			assert.NoError(t, conn.SetDeadline(time.Now().Add(5*time.Second)))

			before := upgradeRequests()
			req := httptest.NewRequest(http.MethodGet, "http://upgrade.test/secret-path?token=secret-query", nil)
			req.Header["Upgrade"] = tc.upgrade
			req.Header["Connection"] = tc.connection
			req.Header["Http2-Settings"] = tc.settings
			req.Header.Set("User-Agent", "upgrade-observer-test")
			req.Header.Set("Authorization", "Bearer secret-authorization")
			req.Header.Set("Cookie", "secret-cookie")
			assert.NoError(t, req.Write(conn))
			reader := bufio.NewReader(conn)
			response, err := http.ReadResponse(reader, req)
			if !assert.NoError(t, err) {
				return
			}
			defer response.Body.Close()
			assert.Equal(t, tc.status, response.StatusCode)
			if response.StatusCode == http.StatusSwitchingProtocols {
				_, err := io.WriteString(conn, http2.ClientPreface)
				assert.NoError(t, err)
				framer := http2.NewFramer(conn, reader)
				assert.NoError(t, framer.WriteSettings())
				var body bytes.Buffer
				for {
					frame, err := framer.ReadFrame()
					if !assert.NoError(t, err) {
						return
					}
					if frame, ok := frame.(*http2.DataFrame); ok {
						assert.Equal(t, uint32(1), frame.StreamID)
						body.Write(frame.Data())
						if frame.StreamEnded() {
							break
						}
					}
				}
				assert.Equal(t, "application response", body.String())
			} else if response.StatusCode == http.StatusOK {
				body, err := io.ReadAll(response.Body)
				assert.NoError(t, err)
				assert.Equal(t, "application response", string(body))
			}

			want := maps.Clone(before)
			if tc.attempts > 0 {
				want[[3]string{transport, tc.settingsHeader, tc.handling}] += tc.attempts
			}
			assert.Equal(t, want, upgradeRequests())
			if tc.attempts == 0 {
				assert.Empty(t, logs.String())
				return
			}
			var event map[string]any
			assert.NoError(t, json.Unmarshal(logs.Bytes(), &event))
			assert.Equal(t, "h2c_upgrade_requested", event["event"])
			assert.Equal(t, transport, event["transport"])
			assert.Equal(t, tc.settingsHeader, event["settings_header"])
			assert.Equal(t, tc.handling, event["handling"])
			assert.Equal(t, server.Listener.Addr().String(), event["listener"])
			assert.Equal(t, "upgrade.test", event["host"])
			assert.Equal(t, "upgrade-observer-test", event["user_agent"])
			assert.Equal(t, conn.LocalAddr().String(), event["remote_addr"])
			assert.NotContains(t, logs.String(), "secret-")
		})
	}

	t.Run("prior knowledge", func(t *testing.T) {
		logs.Reset()
		before := upgradeRequests()
		server := httptest.NewUnstartedServer(nil)
		server.Config = NewHttp2CapableServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, r.Proto)
		}), HTTPOptions{})
		server.Start()
		defer server.Close()
		transport := &http2.Transport{
			AllowHTTP: true,
			DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, network, addr)
			},
		}
		defer transport.CloseIdleConnections()
		client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
		response, err := client.Get(server.URL)
		if !assert.NoError(t, err) {
			return
		}
		defer response.Body.Close()
		body, err := io.ReadAll(response.Body)
		assert.NoError(t, err)
		assert.Equal(t, "HTTP/2.0", string(body))
		assert.Empty(t, logs.String())
		assert.Equal(t, before, upgradeRequests())
	})
}

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

type blockingHealthServer struct {
	grpc_health_v1.UnimplementedHealthServer

	started chan struct{}
	finish  chan struct{}
	calls   atomic.Int32
}

var grpcLameduckTestID atomic.Uint64

func (s *blockingHealthServer) Check(ctx context.Context, _ *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	if s.calls.Add(1) == 1 {
		close(s.started)
	}

	select {
	case <-s.finish:
		return &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_SERVING}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func TestListenAndGracefullyShutdownGRPC_LameduckThenShutdown(t *testing.T) {
	srv := grpc.NewServer()
	health := &blockingHealthServer{
		started: make(chan struct{}),
		finish:  make(chan struct{}),
	}
	grpc_health_v1.RegisterHealthServer(srv, health)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer lis.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		select {
		case <-health.finish:
		default:
			close(health.finish)
		}
	}()

	label := "test-" + strconv.FormatUint(grpcLameduckTestID.Add(1), 10)
	shutdownErr := make(chan error, 1)
	go func() { shutdownErr <- ListenAndGracefullyShutdownGRPC(ctx, label, srv, lis) }()

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	defer conn.Close()
	client := grpc_health_v1.NewHealthClient(conn)

	inflightErr := make(chan error, 1)
	go func() {
		_, err := client.Check(context.Background(), &grpc_health_v1.HealthCheckRequest{})
		inflightErr <- err
	}()

	select {
	case <-health.started:
	case <-time.After(2 * time.Second):
		t.Fatal("in-flight RPC did not start")
	}

	lameduckName := "servercore.grpc." + label
	lameduck, ok := nsgrpc.LameduckFuncsByName[lameduckName]
	if !ok {
		t.Fatalf("lameduck function %q was not registered", lameduckName)
	}

	lameduckReturned := make(chan struct{})
	go func() {
		runShutdownPhases(func() {}, func() map[string]func() {
			return map[string]func(){lameduckName: lameduck}
		}, nil, nil)
		close(lameduckReturned)
	}()
	select {
	case <-lameduckReturned:
	case <-time.After(time.Second):
		t.Fatal("lameduck blocked on the in-flight RPC")
	}

	// Give GOAWAY time to reach the client, then verify that it does not
	// dispatch another RPC on the draining transport.
	time.Sleep(200 * time.Millisecond)
	retryCtx, cancelRetry := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancelRetry()
	if _, err := client.Check(retryCtx, &grpc_health_v1.HealthCheckRequest{}); err == nil {
		t.Fatal("RPC unexpectedly succeeded after lameduck")
	}
	if got := health.calls.Load(); got != 1 {
		t.Fatalf("expected only the in-flight RPC to reach the server, got %d calls", got)
	}

	// Final shutdown should wait on the GracefulStop invocation that lameduck
	// already started, rather than starting a second shutdown lifecycle.
	cancel()
	select {
	case err := <-shutdownErr:
		t.Fatalf("shutdown returned before the in-flight RPC finished: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	close(health.finish)
	select {
	case err := <-inflightErr:
		if err != nil {
			t.Fatalf("in-flight RPC failed during lameduck: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("in-flight RPC did not finish")
	}
	select {
	case err := <-shutdownErr:
		if err != nil {
			t.Fatalf("ListenAndGracefullyShutdownGRPC returned: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown did not return")
	}
}

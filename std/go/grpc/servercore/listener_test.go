// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package servercore

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/soheilhy/cmux"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
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

func TestNewHttp2CapableServer_H2CUpgrade(t *testing.T) {
	finish := make(chan struct{})
	defer func() {
		select {
		case <-finish:
		default:
			close(finish)
		}
	}()
	requestServer := make(chan *http.Server, 1)
	srv := NewHttp2CapableServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ProtoMajor != 2 {
			http.Error(w, "HTTP/2 required", http.StatusHTTPVersionNotSupported)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		requestServer <- r.Context().Value(http.ServerContextKey).(*http.Server)
		if r.URL.Path == "/quick" {
			_, _ = io.WriteString(w, "quick")
			return
		}
		<-finish
		_, _ = w.Write(append([]byte("received: "), body...))
	}), HTTPOptions{HTTP2MaxConcurrentStreams: 7})
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer lis.Close()
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(lis) }()

	conn, err := net.DialTimeout("tcp", lis.Addr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}

	// Pipeline the body and HTTP/2 preface so the upgrade must preserve
	// bytes already buffered by the HTTP/1 reader.
	var handshake bytes.Buffer
	handshake.WriteString("POST /slow HTTP/1.1\r\nHost: localhost\r\nConnection: Upgrade, HTTP2-Settings\r\nUpgrade: h2c\r\nHTTP2-Settings: AAQAAAAB\r\nContent-Length: 12\r\n\r\nupgrade body")
	handshake.WriteString(http2.ClientPreface)
	// Only the Upgrade header sets INITIAL_WINDOW_SIZE=1; the subsequent
	// SETTINGS frame must not mask a failure to apply that header.
	if err := http2.NewFramer(&handshake, nil).WriteSettings(); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Write(handshake.Bytes()); err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(conn)
	response, err := http.ReadResponse(reader, nil)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusSwitchingProtocols || response.Header.Get("Upgrade") != "h2c" {
		t.Fatalf("expected h2c upgrade, got %s with headers %v", response.Status, response.Header)
	}
	select {
	case got := <-requestServer:
		if got != srv {
			t.Error("upgraded request lost its owning http.Server")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("upgraded request did not reach the handler")
	}

	framer := http2.NewFramer(conn, reader)
	framer.ReadMetaHeaders = hpack.NewDecoder(4096, nil)
	sawSettings := false
	readResponse := func(streamID uint32) string {
		t.Helper()
		var body bytes.Buffer
		status := ""
		for {
			frame, err := framer.ReadFrame()
			if err != nil {
				t.Fatalf("reading stream %d: %v", streamID, err)
			}
			switch frame := frame.(type) {
			case *http2.SettingsFrame:
				if !frame.IsAck() {
					if got, ok := frame.Value(http2.SettingMaxConcurrentStreams); !ok || got != 7 {
						t.Fatalf("HTTP/2 stream limit: got %d, present=%t", got, ok)
					}
					sawSettings = true
					if err := framer.WriteSettingsAck(); err != nil {
						t.Fatal(err)
					}
				}
			case *http2.MetaHeadersFrame:
				for _, field := range frame.Fields {
					if field.Name == ":status" {
						status = field.Value
					}
				}
			case *http2.DataFrame:
				if frame.StreamID != streamID {
					t.Fatalf("unexpected response on stream %d, want %d", frame.StreamID, streamID)
				}
				if body.Len() == 0 {
					if len(frame.Data()) != 1 || frame.StreamEnded() {
						t.Fatalf("Upgrade window setting not applied: first DATA has %d bytes, ended=%t", len(frame.Data()), frame.StreamEnded())
					}
					if err := framer.WriteWindowUpdate(streamID, 64); err != nil {
						t.Fatal(err)
					}
				}
				body.Write(frame.Data())
				if frame.StreamEnded() {
					if status != "200" {
						t.Fatalf("stream %d returned status %q", streamID, status)
					}
					return body.String()
				}
			}
		}
	}

	var headers bytes.Buffer
	encoder := hpack.NewEncoder(&headers)
	for _, field := range []hpack.HeaderField{
		{Name: ":method", Value: "GET"},
		{Name: ":scheme", Value: "http"},
		{Name: ":authority", Value: "localhost"},
		{Name: ":path", Value: "/quick"},
	} {
		if err := encoder.WriteField(field); err != nil {
			t.Fatal(err)
		}
	}
	if err := framer.WriteHeaders(http2.HeadersFrameParam{StreamID: 3, BlockFragment: headers.Bytes(), EndHeaders: true, EndStream: true}); err != nil {
		t.Fatal(err)
	}
	if got := readResponse(3); got != "quick" {
		t.Fatalf("unexpected subsequent response: %q", got)
	}
	select {
	case got := <-requestServer:
		if got != srv {
			t.Error("subsequent HTTP/2 request lost its owning http.Server")
		}
	case <-time.After(time.Second):
		t.Fatal("subsequent HTTP/2 request did not reach the handler")
	}

	shutdownErr := make(chan error, 1)
	go func() { shutdownErr <- srv.Shutdown(context.Background()) }()
	for {
		frame, err := framer.ReadFrame()
		if err != nil {
			t.Fatalf("waiting for GOAWAY: %v", err)
		}
		if frame, ok := frame.(*http2.GoAwayFrame); ok {
			if !sawSettings || frame.ErrCode != http2.ErrCodeNo || frame.LastStreamID != 3 {
				t.Fatalf("unexpected GOAWAY: %v (settings received: %t)", frame, sawSettings)
			}
			break
		}
	}

	close(finish)
	if got := readResponse(1); got != "received: upgrade body" {
		t.Fatalf("unexpected in-flight response after GOAWAY: %q", got)
	}
	select {
	case err := <-shutdownErr:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("Shutdown did not return")
	}
	select {
	case err := <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			t.Fatalf("Serve returned: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Serve did not return")
	}
}

func TestNewHttp2CapableServer_HTTP1UpgradeValidation(t *testing.T) {
	server := httptest.NewUnstartedServer(nil)
	server.Config = NewHttp2CapableServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, r.Proto)
	}), HTTPOptions{})
	server.Start()
	defer server.Close()

	for _, tc := range []struct {
		name       string
		upgrade    string
		connection string
		settings   []string
		status     int
	}{
		{name: "ordinary HTTP1", status: http.StatusOK},
		{name: "unrelated upgrade", upgrade: "websocket", connection: "Upgrade", status: http.StatusOK},
		{name: "missing settings connection token", upgrade: "h2c", connection: "Upgrade", settings: []string{""}, status: http.StatusOK},
		{name: "missing settings header", upgrade: "h2c", connection: "Upgrade, HTTP2-Settings", status: http.StatusInternalServerError},
		{name: "duplicate settings", upgrade: "h2c", connection: "Upgrade, HTTP2-Settings", settings: []string{"", ""}, status: http.StatusInternalServerError},
		{name: "invalid settings encoding", upgrade: "h2c", connection: "Upgrade, HTTP2-Settings", settings: []string{"!"}, status: http.StatusInternalServerError},
		{name: "invalid settings length", upgrade: "h2c", connection: "Upgrade, HTTP2-Settings", settings: []string{"AA"}, status: http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, server.URL, nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Upgrade", tc.upgrade)
			req.Header.Set("Connection", tc.connection)
			for _, settings := range tc.settings {
				req.Header.Add("HTTP2-Settings", settings)
			}
			response, err := server.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			body, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatal(err)
			}
			if response.StatusCode != tc.status {
				t.Fatalf("status: got %d, want %d (body %q)", response.StatusCode, tc.status, body)
			}
			if tc.status == http.StatusOK && string(body) != "HTTP/1.1" {
				t.Fatalf("unexpected HTTP/1 response: %q", body)
			}
		})
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

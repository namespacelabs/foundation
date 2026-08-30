// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	executionv2 "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestWaitForBazelCacheReady(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32
	endpoint := startBazelCapabilitiesServer(t, func(ctx context.Context) error {
		if attempts.Add(1) < 3 {
			return status.Error(codes.Unavailable, "starting")
		}
		incoming, _ := metadata.FromIncomingContext(ctx)
		for _, key := range []string{"authorization", "x-nsc-ingress-auth"} {
			values := incoming.Get(key)
			if len(values) != 1 || values[0] != "Bearer readiness-token" {
				return status.Errorf(codes.PermissionDenied, "unexpected %s metadata: %v", key, values)
			}
		}
		return nil
	})

	err := waitForBazelCacheReady(context.Background(), bazelCacheReadinessConfig{
		endpoint:    endpoint,
		bearerToken: "readiness-token",
		waitTimeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("waitForBazelCacheReady: %v", err)
	}
	if attempts.Load() != 3 {
		t.Fatalf("attempts = %d, want 3", attempts.Load())
	}
}

func TestWaitForBazelHTTPCacheReady(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/status" {
			t.Errorf("path = %q, want /status", request.URL.Path)
		}
		if attempts.Add(1) < 3 {
			response.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		for _, key := range []string{"Authorization", "x-nsc-ingress-auth"} {
			if value := request.Header.Get(key); value != "Bearer readiness-token" {
				t.Errorf("%s = %q, want bearer token", key, value)
			}
		}
		response.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	err := waitForBazelCacheReady(context.Background(), bazelCacheReadinessConfig{
		endpoint:    server.URL,
		bearerToken: "readiness-token",
		waitTimeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("waitForBazelCacheReady: %v", err)
	}
	if attempts.Load() != 3 {
		t.Fatalf("attempts = %d, want 3", attempts.Load())
	}
}

func TestWaitForBazelCacheReadyTimesOut(t *testing.T) {
	t.Parallel()

	endpoint := startBazelCapabilitiesServer(t, func(context.Context) error {
		return status.Error(codes.Unavailable, "starting")
	})

	err := waitForBazelCacheReady(context.Background(), bazelCacheReadinessConfig{
		endpoint:    endpoint,
		waitTimeout: 300 * time.Millisecond,
	})
	if err == nil || !strings.Contains(err.Error(), "did not become ready within 300ms") {
		t.Fatalf("waitForBazelCacheReady error = %v, want readiness timeout", err)
	}
}

func TestParseBazelCacheEndpoint(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		endpoint       string
		wantTarget     string
		wantSecure     bool
		wantServerName string
		wantStatusURL  string
	}{
		{endpoint: "grpcs://cache.example.com:444", wantTarget: "cache.example.com:444", wantSecure: true, wantServerName: "cache.example.com"},
		{endpoint: "https://cache.example.com", wantTarget: "cache.example.com:443", wantSecure: true, wantServerName: "cache.example.com", wantStatusURL: "https://cache.example.com/status"},
		{endpoint: "grpc://127.0.0.1:9092", wantTarget: "127.0.0.1:9092", wantSecure: false, wantServerName: "127.0.0.1"},
		{endpoint: "cache.example.com:444", wantTarget: "cache.example.com:444", wantSecure: true, wantServerName: "cache.example.com"},
	} {
		t.Run(test.endpoint, func(t *testing.T) {
			t.Parallel()

			got, err := parseBazelCacheEndpoint(test.endpoint)
			if err != nil {
				t.Fatalf("parseBazelCacheEndpoint: %v", err)
			}
			if got.target != test.wantTarget || got.secure != test.wantSecure || got.serverName != test.wantServerName || got.statusURL != test.wantStatusURL {
				t.Fatalf("parseBazelCacheEndpoint(%q) = %#v, want target=%q secure=%t serverName=%q statusURL=%q", test.endpoint, got, test.wantTarget, test.wantSecure, test.wantServerName, test.wantStatusURL)
			}
		})
	}
}

func startBazelCapabilitiesServer(t *testing.T, check func(context.Context) error) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := grpc.NewServer(grpc.UnknownServiceHandler(func(_ any, stream grpc.ServerStream) error {
		method, _ := grpc.Method(stream.Context())
		if method != bazelGetCapabilitiesMethod {
			return status.Errorf(codes.Unimplemented, "unexpected method %q", method)
		}
		request := &executionv2.GetCapabilitiesRequest{}
		if err := stream.RecvMsg(request); err != nil {
			return err
		}
		if err := check(stream.Context()); err != nil {
			return err
		}
		return stream.SendMsg(&executionv2.ServerCapabilities{})
	}))
	t.Cleanup(func() {
		server.Stop()
		listener.Close()
	})
	go func() {
		_ = server.Serve(listener)
	}()

	return "grpc://" + listener.Addr().String()
}

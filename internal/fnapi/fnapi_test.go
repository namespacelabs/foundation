// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fnapi

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync/atomic"
	"testing"

	"gotest.tools/assert"
)

// resetOnceServer resets the connection of the first request it receives, and
// answers every later request with an empty JSON object.
func resetOnceServer(t *testing.T) (*httptest.Server, *atomic.Int32) {
	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) > 1 {
			_, _ = w.Write([]byte("{}"))
			return
		}

		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Errorf("hijack: %v", err)
			return
		}

		// A zero linger makes Close send a RST, so the client sees ECONNRESET.
		_ = conn.(*net.TCPConn).SetLinger(0)
		_ = conn.Close()
	}))
	t.Cleanup(srv.Close)

	return srv, &calls
}

func staticEndpoint(url string) ResolveFunc {
	return func(context.Context, ResolvedToken) (string, error) {
		return url, nil
	}
}

func TestRetryableCallRetriesConnectionReset(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("resets surface as WSAECONNRESET on Windows, which is not retried")
	}

	srv, calls := resetOnceServer(t)

	err := Call[struct{}]{Method: "test.Service/Method", Retryable: true}.Do(context.Background(), struct{}{}, staticEndpoint(srv.URL), nil)
	assert.Equal(t, calls.Load(), int32(2), "error: %v", err)
	assert.NilError(t, err)
}

func TestNonRetryableCallDoesNotRetryConnectionReset(t *testing.T) {
	srv, calls := resetOnceServer(t)

	err := Call[struct{}]{Method: "test.Service/Method"}.Do(context.Background(), struct{}{}, staticEndpoint(srv.URL), nil)
	assert.Assert(t, err != nil)
	assert.Equal(t, calls.Load(), int32(1))
}

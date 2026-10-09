// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package kubeclient

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"k8s.io/client-go/rest"
)

func newTestREST(t *testing.T, handler http.HandlerFunc) *REST {
	t.Helper()

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	cli, err := NewREST(&rest.Config{Host: srv.URL})
	if err != nil {
		t.Fatal(err)
	}

	return cli
}

type object struct {
	Name string `json:"name"`
}

func TestGetRetriesAfterRetryAfter(t *testing.T) {
	var calls atomic.Int32

	cli := newTestREST(t, func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		fmt.Fprint(w, `{"name":"ok"}`)
	})

	var got object
	if err := cli.Get(context.Background(), "/api/v1/thing", &got); err != nil {
		t.Fatal(err)
	}

	if got.Name != "ok" {
		t.Errorf("got name %q, want %q", got.Name, "ok")
	}

	if n := calls.Load(); n != 2 {
		t.Errorf("got %d requests, want 2", n)
	}
}

func TestGetDoesNotRetryWithoutRetryAfter(t *testing.T) {
	var calls atomic.Int32

	cli := newTestREST(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	})

	var got object
	err := cli.Get(context.Background(), "/api/v1/thing", &got)
	if err == nil {
		t.Fatal("got nil error, want a failure")
	}

	if n := calls.Load(); n != 1 {
		t.Errorf("got %d requests, want 1", n)
	}
}

func TestGetRetriesConnectionReset(t *testing.T) {
	var calls atomic.Int32

	cli := newTestREST(t, func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			// Hijack and close without a response, so that the client
			// observes the connection going away mid-request.
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Error(err)
				return
			}
			conn.Close()
			return
		}

		fmt.Fprint(w, `{"name":"ok"}`)
	})

	var got object
	if err := cli.Get(context.Background(), "/api/v1/thing", &got); err != nil {
		t.Fatal(err)
	}

	if got.Name != "ok" {
		t.Errorf("got name %q, want %q", got.Name, "ok")
	}

	if n := calls.Load(); n != 2 {
		t.Errorf("got %d requests, want 2", n)
	}
}

func TestGetNotFound(t *testing.T) {
	cli := newTestREST(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no such thing", http.StatusNotFound)
	})

	var got object
	err := cli.Get(context.Background(), "/api/v1/thing", &got)

	var status *StatusError
	if !errors.As(err, &status) {
		t.Fatalf("got %v, want a *StatusError", err)
	}

	if !status.IsNotFound() {
		t.Errorf("got code %d, want 404", status.Code)
	}
}

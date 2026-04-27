// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package download

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"namespacelabs.dev/foundation/internal/artifacts"
	"namespacelabs.dev/foundation/internal/bytestream"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/tasks"
)

func TestRetriesTransientEOF(t *testing.T) {
	body := []byte("hello world, this is the payload")

	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := requests.Add(1)
		// Fail the first 2 attempts with a truncated body.
		if n <= 2 {
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(body[:5])
			// Hijack to abort the connection so the client surfaces io.ErrUnexpectedEOF
			// rather than getting a clean close.
			if hj, ok := w.(http.Hijacker); ok {
				if conn, _, err := hj.Hijack(); err == nil {
					_ = conn.Close()
					return
				}
			}
			return
		}
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	defer server.Close()

	hash := sha256.Sum256(body)
	ref := artifacts.Reference{
		URL:    server.URL,
		Digest: schema.Digest{Algorithm: "sha256", Hex: hex.EncodeToString(hash[:])},
	}

	ctx := tasks.WithSink(context.Background(), tasks.NullSink())

	var got []byte
	err := compute.Do(ctx, func(ctx context.Context) error {
		bs, err := compute.GetValue(ctx, URL(ref))
		if err != nil {
			return err
		}
		got, err = bytestream.ReadAll(bs)
		return err
	})
	if err != nil {
		t.Fatalf("download failed: %v", err)
	}
	if string(got) != string(body) {
		t.Fatalf("got %q, want %q", got, body)
	}
	if requests.Load() != 3 {
		t.Fatalf("want 3 requests (2 transient failures + 1 success), got %d", requests.Load())
	}
}

func TestNonRetryable4xx(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		http.Error(w, "nope", http.StatusForbidden)
	}))
	defer server.Close()

	ctx := tasks.WithSink(context.Background(), tasks.NullSink())

	err := compute.Do(ctx, func(ctx context.Context) error {
		_, err := compute.GetValue(ctx, UnverifiedURL(server.URL))
		return err
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if requests.Load() != 1 {
		t.Fatalf("want 1 request (4xx is permanent), got %d", requests.Load())
	}
}

func TestRetries5xx(t *testing.T) {
	body := []byte("payload")

	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := requests.Add(1)
		if n == 1 {
			http.Error(w, "boom", http.StatusBadGateway)
			return
		}
		_, _ = w.Write(body)
	}))
	defer server.Close()

	ctx := tasks.WithSink(context.Background(), tasks.NullSink())

	var got []byte
	err := compute.Do(ctx, func(ctx context.Context) error {
		bs, err := compute.GetValue(ctx, UnverifiedURL(server.URL))
		if err != nil {
			return err
		}
		got, err = bytestream.ReadAll(bs)
		return err
	})
	if err != nil {
		t.Fatalf("download failed: %v", err)
	}
	if string(got) != string(body) {
		t.Fatalf("got %q, want %q", got, body)
	}
	if requests.Load() != 2 {
		t.Fatalf("want 2 requests (1 502 + 1 success), got %d", requests.Load())
	}
}

func TestErrorClassification(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"unexpected EOF", io.ErrUnexpectedEOF, true},
		{"plain EOF", io.EOF, true},
		{"context canceled", context.Canceled, false},
		{"context deadline", context.DeadlineExceeded, false},
		{"http 502", newHTTPStatusError("u", http.StatusBadGateway), true},
		{"http 503", newHTTPStatusError("u", http.StatusServiceUnavailable), true},
		{"http 429", newHTTPStatusError("u", http.StatusTooManyRequests), true},
		{"http 403", newHTTPStatusError("u", http.StatusForbidden), false},
		{"http 404", newHTTPStatusError("u", http.StatusNotFound), false},
		{"digest mismatch", newDigestMismatchError("u", "a", "b"), true},
		{"random error", errors.New("foo"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isRetryableDownloadError(tc.err); got != tc.want {
				t.Fatalf("isRetryableDownloadError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

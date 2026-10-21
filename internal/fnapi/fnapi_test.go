// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fnapi

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"gotest.tools/assert"
)

func TestCallNetworkTimeout(t *testing.T) {
	for _, test := range []struct {
		name      string
		retryable bool
		deadline  time.Duration
	}{
		{name: "retry enabled", retryable: true},
		{name: "retry disabled"},
		{name: "caller deadline", retryable: true, deadline: 10 * time.Millisecond},
	} {
		t.Run(test.name, func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.Copy(io.Discard, r.Body)
				if requests.Add(1) == 1 {
					<-r.Context().Done()
					return
				}
				_, _ = io.WriteString(w, "ready")
			}))
			defer server.Close()

			transport := http.DefaultTransport.(*http.Transport).Clone()
			transport.ResponseHeaderTimeout = 50 * time.Millisecond
			previousTransport := http.DefaultTransport
			http.DefaultTransport = transport
			defer func() {
				http.DefaultTransport = previousTransport
				transport.CloseIdleConnections()
			}()

			ctx := context.Background()
			if test.deadline > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, test.deadline)
				defer cancel()
			}

			var body string
			err := (Call[struct{}]{Method: "lookup", Retryable: test.retryable}).Do(ctx, struct{}{},
				func(context.Context, ResolvedToken) (string, error) { return server.URL, nil },
				func(r io.Reader) error {
					data, err := io.ReadAll(r)
					body = string(data)
					return err
				})

			switch {
			case test.deadline > 0:
				assert.Assert(t, errors.Is(err, context.DeadlineExceeded))
			case !test.retryable:
				var netErr net.Error
				assert.Assert(t, errors.As(err, &netErr))
				assert.Assert(t, netErr.Timeout())
			default:
				assert.NilError(t, err)
				assert.Equal(t, body, "ready")
			}
		})
	}
}

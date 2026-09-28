// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package kubeclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/client-go/rest"
)

// REST is a minimal Kubernetes API client. It exists so that callers which only
// need a handful of requests don't have to link the typed clientset, which
// carries every built-in API group with it.
type REST struct {
	host string
	cli  *http.Client
}

func NewREST(cfg *rest.Config) (*REST, error) {
	cli, err := rest.HTTPClientFor(cfg)
	if err != nil {
		return nil, err
	}

	return &REST{host: strings.TrimSuffix(cfg.Host, "/"), cli: cli}, nil
}

// maxRetries matches the default that client-go's rest.Request applies; the
// bare http.Client from rest.HTTPClientFor carries no retries of its own.
const maxRetries = 10

func (r *REST) do(ctx context.Context, path string, query url.Values) (*http.Response, error) {
	u := r.host + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}

		req.Header.Set("Accept", "application/json")

		resp, err := r.cli.Do(req)

		if wait, ok := retryAfter(resp, err, attempt); ok {
			drainAndClose(resp)

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(wait):
			}

			continue
		}

		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			resp.Body.Close()
			return nil, &StatusError{Path: path, Code: resp.StatusCode, Body: strings.TrimSpace(string(body))}
		}

		return resp, nil
	}
}

// retryAfter mirrors how client-go decides to retry a request: connection-level
// failures are retried after a second, and a 429 or 5xx only when the server
// asks for it with a Retry-After header.
func retryAfter(resp *http.Response, err error, attempt int) (time.Duration, bool) {
	if attempt >= maxRetries {
		return 0, false
	}

	if err != nil {
		if utilnet.IsConnectionReset(err) || utilnet.IsProbableEOF(err) || utilnet.IsHTTP2ConnectionLost(err) {
			return time.Second, true
		}

		return 0, false
	}

	if resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode < 500 {
		return 0, false
	}

	seconds, err := strconv.Atoi(resp.Header.Get("Retry-After"))
	if err != nil || seconds < 0 {
		return 0, false
	}

	return time.Duration(seconds) * time.Second, true
}

// drainAndClose consumes a bounded amount of the body so that the connection
// can be reused by the retry.
func drainAndClose(resp *http.Response) {
	if resp == nil {
		return
	}

	defer resp.Body.Close()

	const maxSlurp = 2 << 10
	io.Copy(io.Discard, io.LimitReader(resp.Body, maxSlurp))
}

type StatusError struct {
	Path string
	Code int
	Body string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("kubernetes: GET %s: %d: %s", e.Path, e.Code, e.Body)
}

func (e *StatusError) IsNotFound() bool { return e.Code == http.StatusNotFound }

// Get decodes the object at path into out.
func (r *REST) Get(ctx context.Context, path string, out any) error {
	resp, err := r.do(ctx, path, nil)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	return json.NewDecoder(resp.Body).Decode(out)
}

// Watch streams a watch on path, handing each event's object to callback until
// it returns true, the stream ends, or ctx is cancelled.
func (r *REST) Watch(ctx context.Context, path string, query url.Values, callback func(json.RawMessage) (bool, error)) error {
	q := url.Values{}
	for k, v := range query {
		q[k] = v
	}
	q.Set("watch", "true")

	resp, err := r.do(ctx, path, q)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	dec := json.NewDecoder(resp.Body)
	for {
		var ev struct {
			Type   string          `json:"type"`
			Object json.RawMessage `json:"object"`
		}

		if err := dec.Decode(&ev); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		done, err := callback(ev.Object)
		if err != nil || done {
			return err
		}
	}
}

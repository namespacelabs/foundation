// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/std/tasks"
)

// fakeInstanceServer creates instance "inst-1" and records the instances it is
// asked to destroy.
func fakeInstanceServer(t *testing.T) func() []string {
	var mu sync.Mutex
	var destroyed []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/CreateInstance"):
			_, _ = w.Write([]byte(`{"instanceId":"inst-1"}`))

		case strings.HasSuffix(r.URL.Path, "/DestroyKubernetesCluster"):
			var req DestroyKubernetesClusterRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Errorf("decoding destroy request: %v", err)
			}

			mu.Lock()
			destroyed = append(destroyed, req.ClusterId)
			mu.Unlock()

			_, _ = w.Write([]byte(`{}`))

		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("NSC_ENDPOINT", srv.URL)

	return func() []string {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(destroyed)
	}
}

func TestCreateClusterDestroysInstanceAfterInterrupt(t *testing.T) {
	destroyed := fakeInstanceServer(t)

	testAPI := API{
		CreateInstance:           fnapi.Call[CreateInstanceRequest]{Method: "test/CreateInstance"},
		DestroyKubernetesCluster: fnapi.Call[DestroyKubernetesClusterRequest]{Method: "test/DestroyKubernetesCluster"},
	}

	// Mirror the CLI: a throttler in the context, and Ctrl-C cancelling the parent.
	ctx := tasks.ContextWithThrottler(tasks.WithSink(context.Background(), tasks.NullSink()), io.Discard, &tasks.ThrottleConfigurations{})
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	err := compute.Do(ctx, func(ctx context.Context) error {
		if _, err := CreateCluster(ctx, testAPI, CreateInstanceOpts{}); err != nil {
			return err
		}

		cancel()
		return ctx.Err()
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("compute.Do() error = %v, want context.Canceled", err)
	}

	if got := destroyed(); !slices.Equal(got, []string{"inst-1"}) {
		t.Fatalf("destroyed instances = %v, want [inst-1]", got)
	}
}

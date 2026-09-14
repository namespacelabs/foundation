// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package download

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"gotest.tools/assert"
	"namespacelabs.dev/foundation/internal/artifacts"
	"namespacelabs.dev/foundation/internal/bytestream"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/tasks"
)

func TestDownloadCache(t *testing.T) {
	previous := dirs.CacheDir
	dirs.CacheDir = t.TempDir()
	t.Cleanup(func() { dirs.CacheDir = previous })
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		fmt.Fprint(w, r.URL.Path)
	}))
	defer server.Close()

	refFor := func(body string) artifacts.Reference {
		h := sha256.Sum256([]byte(body))
		return artifacts.Reference{URL: server.URL + body, Digest: schema.Digest{Algorithm: "sha256", Hex: hex.EncodeToString(h[:])}}
	}
	ctx := tasks.WithSink(context.Background(), tasks.NullSink())
	read := func(ref artifacts.Reference, opts ...Option) error {
		return compute.Do(ctx, func(ctx context.Context) error {
			bs, err := compute.GetValue(ctx, URL(ref, opts...))
			if err != nil {
				return err
			}
			got, err := bytestream.ReadAll(bs)
			if err != nil {
				return err
			}
			if string(got) != ref.URL[len(server.URL):] {
				return fmt.Errorf("unexpected contents: %q", got)
			}
			return nil
		})
	}

	ref := refFor("/linux-amd64-v1")
	var wg sync.WaitGroup
	errs := make(chan error, 4)
	for range 4 {
		wg.Go(func() { errs <- read(ref, WithCache()) })
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		assert.NilError(t, err)
	}
	assert.Equal(t, requests.Load(), int32(1))
	assert.NilError(t, read(ref, WithCache()))
	assert.Equal(t, requests.Load(), int32(1))

	// A cached SDK must not turn an ordinary download into a cache hit.
	assert.NilError(t, read(ref))
	assert.NilError(t, read(ref))
	assert.Equal(t, requests.Load(), int32(3))

	assert.NilError(t, read(refFor("/linux-arm64-v1"), WithCache()))
	assert.NilError(t, read(refFor("/linux-amd64-v2"), WithCache()))
	assert.Equal(t, requests.Load(), int32(5))

	dir, err := dirs.Subdir("sdk-downloads")
	assert.NilError(t, err)
	path := filepath.Join(dir, ref.Digest.Algorithm, ref.Digest.Hex)
	assert.NilError(t, os.WriteFile(path, []byte("corrupted"), 0600))
	assert.NilError(t, read(ref, WithCache()))
	assert.Equal(t, requests.Load(), int32(6))

	server.Close()
	assert.NilError(t, read(ref, WithCache()))
	assert.Equal(t, requests.Load(), int32(6))
}

// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package kubectl_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/assert"
	"namespacelabs.dev/foundation/internal/artifacts"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnfs/fsdigest"
	"namespacelabs.dev/foundation/internal/sdk/host"
	"namespacelabs.dev/foundation/internal/sdk/kubectl"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/tasks"
)

func init() {
	compute.RegisterByteDigesters()
	fsdigest.Register()
}

func TestSDKDownloadCache(t *testing.T) {
	previous := dirs.CacheDir
	dirs.CacheDir = t.TempDir()
	t.Cleanup(func() { dirs.CacheDir = previous })
	body := []byte("#!/bin/sh\necho sdk-cache-test\n")

	for _, archive := range []bool{false, true} {
		name := "binary"
		payload := body
		if archive {
			name = "archive"
			var buf bytes.Buffer
			gz := gzip.NewWriter(&buf)
			tw := tar.NewWriter(gz)
			assert.NilError(t, tw.WriteHeader(&tar.Header{Name: "sdk/bin/tool", Mode: 0755, Size: int64(len(body))}))
			_, err := tw.Write(body)
			assert.NilError(t, err)
			assert.NilError(t, tw.Close())
			assert.NilError(t, gz.Close())
			payload = buf.Bytes()
		}
		t.Run(name, func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				w.Write(payload)
			}))
			defer server.Close()
			h := sha256.Sum256(payload)
			ref := artifacts.Reference{URL: server.URL, Digest: schema.Digest{Algorithm: "sha256", Hex: hex.EncodeToString(h[:])}}
			platform := specs.Platform{OS: "test", Architecture: "cache"}
			kubectl.Pins["test/cache"] = ref
			defer delete(kubectl.Pins, "test/cache")
			ctx := tasks.WithSink(context.Background(), tasks.NullSink())
			ensure := func() string {
				var path string
				assert.NilError(t, compute.Do(ctx, func(ctx context.Context) error {
					if archive {
						sdk, err := compute.GetValue(ctx, &host.PrepareSDK{Name: "test", Platform: platform, Binary: "sdk/bin/tool", Version: "1", Ref: ref})
						path = sdk.Binary
						return err
					}
					sdk, err := kubectl.EnsureSDK(ctx, platform)
					path = string(sdk)
					return err
				}))
				got, err := os.ReadFile(path)
				assert.NilError(t, err)
				assert.Equal(t, string(got), string(body))
				info, err := os.Stat(path)
				assert.NilError(t, err)
				assert.Equal(t, info.Mode().Perm(), os.FileMode(0755))
				return path
			}
			path := ensure()
			server.Close()
			assert.Equal(t, ensure(), path)
			assert.NilError(t, os.WriteFile(path, []byte("corrupted executable"), 0755))
			assert.Equal(t, ensure(), path)
			assert.Equal(t, requests.Load(), int32(1))
		})
	}
}

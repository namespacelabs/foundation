// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package golang

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/compute/cache"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/foundation/std/tasks/simplelog"
)

func TestSourceDigest(t *testing.T) {
	dir := t.TempDir()
	filename := filepath.Join(dir, "main.go")
	if err := os.WriteFile(filename, []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	// Snapshots ignore symlinks, including dangling ones.
	if err := os.Symlink("missing", filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}

	ctx := tasks.WithSink(context.Background(), simplelog.NewSink(io.Discard, 0))
	readDigest := func() schema.Digest {
		t.Helper()
		var digest schema.Digest
		err := compute.DoWithCache(ctx, cache.NoCache, func(ctx context.Context) error {
			result, err := compute.Get(ctx, deferSourceDigest(os.DirFS(dir)))
			if err != nil {
				return err
			}
			digest = result.Value
			if !digest.IsSet() || digest != result.Digest {
				t.Errorf("invalid result: value %v, digest %v", digest, result.Digest)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		return digest
	}

	before := readDigest()
	snapshot, err := memfs.Snapshot(os.DirFS(dir), memfs.SnapshotOpts{})
	if err != nil {
		t.Fatal(err)
	}
	want, err := snapshot.ComputeDigest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if before != want || readDigest() != before {
		t.Error("source digest must match the snapshot and be stable across invocations")
	}

	if err := os.WriteFile(filename, []byte("package main\nfunc main() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	after := readDigest()
	if before == after {
		t.Error("a later invocation must see edited sources")
	}
	if err := os.Rename(filename, filepath.Join(dir, "renamed.go")); err != nil {
		t.Fatal(err)
	}
	renamed := readDigest()
	if after == renamed {
		t.Error("file names are part of the source digest")
	}
	if err := os.Remove(filepath.Join(dir, "renamed.go")); err != nil {
		t.Fatal(err)
	}
	if renamed == readDigest() {
		t.Error("a later invocation must see deleted sources")
	}
}

func TestSourceDigestReadError(t *testing.T) {
	ctx := tasks.WithSink(context.Background(), simplelog.NewSink(io.Discard, 0))
	err := compute.DoWithCache(ctx, cache.NoCache, func(ctx context.Context) error {
		_, err := compute.GetValue(ctx, deferSourceDigest(os.DirFS(filepath.Join(t.TempDir(), "missing"))))
		return err
	})
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("got %v, want a missing-source error", err)
	}
}

func BenchmarkSourceDigestRetention(b *testing.B) {
	dir := b.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "source"), make([]byte, 1<<20), 0600); err != nil {
		b.Fatal(err)
	}
	ctx := tasks.WithSink(context.Background(), simplelog.NewSink(io.Discard, 0))
	for _, keepSnapshot := range []bool{true, false} {
		name := "digest"
		if keepSnapshot {
			name = "snapshot"
		}
		b.Run(name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				err := compute.DoWithCache(ctx, cache.NoCache, func(ctx context.Context) error {
					const builds = 64
					nodes := make([]compute.UntypedComputable, 0, builds)
					runtime.GC()
					var before, after runtime.MemStats
					runtime.ReadMemStats(&before)
					for j := 0; j < builds; j++ {
						if keepSnapshot {
							node := memfs.DeferSnapshot(os.DirFS(dir), memfs.SnapshotOpts{})
							if _, err := compute.GetValue(ctx, node); err != nil {
								return err
							}
							nodes = append(nodes, node)
						} else {
							node := deferSourceDigest(os.DirFS(dir))
							if _, err := compute.GetValue(ctx, node); err != nil {
								return err
							}
							nodes = append(nodes, node)
						}
					}
					runtime.GC()
					runtime.ReadMemStats(&after)
					runtime.KeepAlive(nodes)
					b.ReportMetric(float64(int64(after.HeapAlloc)-int64(before.HeapAlloc))/builds, "retained-B/build")
					return nil
				})
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

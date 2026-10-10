// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package compute

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"

	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/foundation/std/tasks/simplelog"
)

func TestDoRemovesTemporaryArtifactStore(t *testing.T) {
	ctx := tasks.WithSink(context.Background(), simplelog.NewSink(io.Discard, 0))
	var path string
	if err := Do(ctx, func(ctx context.Context) error {
		store := ArtifactStore(ctx)
		digest := schema.Digest{Algorithm: "sha256", Hex: "artifact"}
		if err := store.WriteBytes(ctx, digest, []byte("contents")); err != nil {
			return err
		}
		blob, err := store.Blob(digest)
		if err != nil {
			return err
		}
		path = blob.(*os.File).Name()
		blob.Close()
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("temporary artifact remained: %v", err)
	}
}

func TestDoRunsCleanupsAfterParentIsCancelled(t *testing.T) {
	// Mirror the CLI: a throttler in the context, and Ctrl-C cancelling the parent.
	ctx := tasks.ContextWithThrottler(tasks.WithSink(context.Background(), simplelog.NewSink(io.Discard, 0)), io.Discard, &tasks.ThrottleConfigurations{})
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var ran, bounded bool
	var cleanupCtxErr error
	err := Do(ctx, func(ctx context.Context) error {
		On(ctx).Cleanup(tasks.Action("test.cleanup"), func(ctx context.Context) error {
			ran = true
			cleanupCtxErr = ctx.Err()
			_, bounded = ctx.Deadline()
			return nil
		})

		cancel()
		return ctx.Err()
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Do() error = %v, want context.Canceled", err)
	}
	if !ran {
		t.Fatal("cleanup did not run")
	}
	if cleanupCtxErr != nil {
		t.Fatalf("cleanup ran with a done context: %v", cleanupCtxErr)
	}
	if !bounded {
		t.Fatal("cleanup context has no deadline")
	}
}

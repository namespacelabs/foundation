// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package compute

import (
	"context"
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

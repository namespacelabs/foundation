// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package digestfs

import (
	"context"
	"crypto/sha256"
	"io/fs"

	"namespacelabs.dev/foundation/internal/fnfs/maketarfs"
	"namespacelabs.dev/foundation/schema"
)

func Digest(ctx context.Context, fsys fs.FS) (schema.Digest, error) {
	return DigestWithOpts(ctx, fsys, nil, nil)
}

func DigestWithOpts(ctx context.Context, fsys fs.FS, includeFiles []string, excludeFiles []string) (schema.Digest, error) {
	h := sha256.New()
	// Assumes that TarFS() produces a reproducible output.
	err := maketarfs.TarFS(ctx, h, fsys, includeFiles, excludeFiles)
	return schema.FromHash("sha256", h), err
}

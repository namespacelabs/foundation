// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package digestfs

import (
	"context"
	"crypto/sha256"
	"io/fs"

	"namespacelabs.dev/foundation/internal/fnfs/maketarfs"
	"namespacelabs.dev/foundation/schema"
)

func Digest(ctx context.Context, fsys fs.FS, includeFiles []string, excludeFiles []string) (schema.Digest, error) {
	h := sha256.New()
	err := maketarfs.TarFS(ctx, h, fsys, includeFiles, excludeFiles)
	return schema.FromHash("sha256", h), err
}

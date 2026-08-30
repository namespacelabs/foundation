// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fnfs

import (
	"context"
	"io/fs"

	"namespacelabs.dev/foundation/internal/bytestream"
)

type TotalSizeFS interface {
	TotalSize(ctx context.Context) (uint64, error)
}

func TotalSize(ctx context.Context, fsys fs.FS) (uint64, error) {
	if tsf, ok := fsys.(TotalSizeFS); ok {
		return tsf.TotalSize(ctx)
	}

	var count uint64
	err := VisitFiles(ctx, fsys, func(path string, contents bytestream.ByteStream, de fs.DirEntry) error {
		count += contents.ContentLength()
		return nil
	})
	return count, err
}

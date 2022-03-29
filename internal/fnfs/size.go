// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package fnfs

import (
	"context"
	"io/fs"
)

type TotalSizeFS interface {
	TotalSize(ctx context.Context) (uint64, error)
}

func TotalSize(ctx context.Context, fsys fs.FS) (uint64, error) {
	if tsf, ok := fsys.(TotalSizeFS); ok {
		return tsf.TotalSize(ctx)
	}

	var count uint64
	err := VisitFiles(ctx, fsys, func(path string, contents []byte, de fs.DirEntry) error {
		count += uint64(len(contents))
		return nil
	})
	return count, err
}